package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dave/jennifer/jen"
	. "github.com/dave/jennifer/jen"
	"github.com/fatih/structtag"
	"github.com/urfave/cli/v2"
	"github.com/wricardo/structparser"
)

const inputName = "input"
const receiverId = "x"

func generate(c *cli.Context) error {
	parsed, err := structparser.ParseDirectoryWithFilter(c.String("input"), nil)
	if err != nil {
		return err
	}

	if c.Bool("verbose") {
		pretty, _ := json.MarshalIndent(parsed, "", "\t")
		log.Println("parsed structs", string(pretty))
	}

	{
		f := NewFile(c.String("package"))
		f.PackageComment("Code generated by generator, DO NOT EDIT.")
		f.Id("import").Parens(Id("mongopagination \"github.com/gobeam/mongo-go-pagination\""))

		for _, strct := range parsed {

			metadata := structMetadata{}
			if len(strct.Docs) == 1 {
				if err := json.Unmarshal([]byte(strct.Docs[0]), &metadata); err != nil {
					return err
				}
			}
			if metadata.CollectionName == "" {
				if c.Bool("verbose") {
					log.Printf("skipping %s because there is no collection_name in json struct documentation\n", strct.Name)
				}
				continue
			}

			var idField *structparser.Field
			tagToFieldMap := make(map[string]structparser.Field)

			// validate _id
			for k, v := range strct.Fields {
				if v.Tag != "" {
					val, err := structtag.Parse(v.Tag)
					if err != nil {
						return err
					}
					bsonTag, ok := val.Get("bson")
					if ok != nil {
						return fmt.Errorf("field %s on struct %s has no bson tag", strct.Name, v.Name)
					}
					if bsonTag.Name == "" {
						return fmt.Errorf("field %s on struct %s has invalid bson tag. The first part of the tag cannot be empty. It may be - for ignored field, but not empty", strct.Name, v.Name)
					}
					if bsonTag.Name == "_id" {
						idField = &strct.Fields[k]
					}
					tagToFieldMap[bsonTag.Name] = strct.Fields[k]
				}
			}

			// repository struct
			repositoryName := makeFirstLowerCase(strct.Name) + "Repository"
			f.Type().Id(repositoryName).Struct(
				Id("db").Id("*mongo.Database"),
			)

			structReceiver := Id(receiverId).Id("*" + repositoryName)

			//constructor
			f.Func().Id("New" + strct.Name + "Repository").Params(Id("db").Op("*").Qual("go.mongodb.org/mongo-driver/mongo", "Database")).Id(strct.Name + "Repository").Block(
				Return(Op("&").Id(repositoryName).Values(Dict{
					Id("db"): Id("db"),
				})),
			)

			// getColletion
			fn := f.Func().Params(structReceiver).Id("getCollection").Params().Op("*").Id("mongo.Collection")
			fn.Block(
				Return(Id(receiverId + ".db.Collection").Params(Lit(metadata.CollectionName))),
			)

			// insert
			fn = f.Func().Params(structReceiver).Id("Insert").Params(Id("ctx").Qual("context", "Context"), Id(inputName).Op("*").Id(strct.Name)).Id("error")
			fn.BlockFunc(func(g *Group) {
				g.If(
					Id(inputName + ".ID.IsZero").Call(),
				).Block(
					Id(inputName+".ID").Op("=").Qual("go.mongodb.org/mongo-driver/bson/primitive", "NewObjectID").Call(),
				)
				g.If(
					Id(inputName + ".CreatedAt.IsZero").Call(),
				).Block(
					Id(inputName+".CreatedAt").Op("=").Qual("time", "Now").Call(),
				)

				g.If(
					List(Id("res"), Err()).Op(":=").Id(receiverId).Dot("getCollection").Call().Dot("InsertOne").Call(List(Id("ctx"), Id(inputName))),
					Err().Op("!=").Nil(),
				).Block(
					Return(Err()),
				).Else().If(
					Id("res").Dot("InsertedID").Op("!=").Nil(),
				).Block(
					Return(Nil()),
				)
				g.Return(Nil())

			})

			// getPrimitive
			if idField != nil {
				fn = f.Func().Params(structReceiver).Id("GetPrimitive").Params(Id("ctx").Qual("context", "Context"), Id(getVarNameForField(*idField)).Id(idField.Type)).Parens(List(Op("*").Id(strct.Name), Error()))
				fn.BlockFunc(func(g *Group) {
					g.Id("res").Op(":=").Id(strct.Name).Values()
					g.Err().Op(":=").Id(receiverId).Dot("getCollection").Call().Dot("FindOne").Call(Id("ctx"), Qual("go.mongodb.org/mongo-driver/bson", "D").Values(fieldToBson(*idField, Id(getVarNameForField(*idField))))).Dot("Decode").Call(Op("&").Id("res"))
					isErrorNoDocuments(g)
					g.Return(Op("&").Id("res"), Nil())
				})
				fn = f.Func().Params(structReceiver).Id("Get").Params(Id("ctx").Qual("context", "Context"), Id(getVarNameForField(*idField)).String()).Parens(List(Op("*").Id(strct.Name), Error()))
				fn.BlockFunc(func(g *Group) {
					g.List(Id("_id"), Err()).Op(":=").Qual("go.mongodb.org/mongo-driver/bson/primitive", "ObjectIDFromHex").Call(Id(getVarNameForField(*idField)))
					isErrorReturnNilErr(g)
					g.Return(Id(receiverId).Dot("GetPrimitive").Call(Id("ctx"), Id("_id")))
				})
				fn = f.Func().Params(structReceiver).Id("Delete").Params(Id("ctx").Qual("context", "Context"), Id(getVarNameForField(*idField)).Id(idField.Type)).Parens(List(Op("*").Qual("go.mongodb.org/mongo-driver/mongo", "DeleteResult"), Error()))
				fn.BlockFunc(func(g *Group) {
					g.List(Id("res"), Err()).Op(":=").Id(receiverId).Dot("getCollection").Call().Dot("DeleteOne").Call(Id("ctx"), Qual("go.mongodb.org/mongo-driver/bson", "D").Values(fieldToBson(*idField, Id(getVarNameForField(*idField)))))
					isErrorNoDocuments(g)
					g.Return(Id("res"), Nil())
				})
			}

			// list
			fn = f.Func().Params(structReceiver).Id("List").Params(Id("ctx").Qual("context", "Context")).Parens(List(Index().Id(strct.Name), Error()))
			fn.BlockFunc(func(g *Group) {
				g.List(Id("cur"), Err()).Op(":=").Id(receiverId).Dot("getCollection").Call().Dot("Find").Call(Id("ctx"), Qual("go.mongodb.org/mongo-driver/bson", "D").Values())
				isErrorReturnNilErr(g)
				g.Id("res").Op(":=").Index().Id(strct.Name).Values()
				g.If(Err().Op(":=").Id("cur").Dot("All").Call(Id("ctx"), Op("&").Id("res")).Op(";").Id("err").Op("!=").Nil()).Block(
					Return(Nil(), Err()),
				)
				g.Return(Id("res"), Nil())
			})

			// indexes
			generateIndexes(f, strct, metadata, tagToFieldMap, structReceiver)

		}

		file, err := os.OpenFile(c.String("output"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}

		if c.Bool("verbose") {
			f.Render(os.Stdout)
		}
		err = f.Render(file)
		if err != nil {
			return err
		}
		file.Close()

		generateInterfaces(c, f)

		file, err = os.OpenFile(c.String("output"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer file.Close()

		if c.Bool("verbose") {
			f.Render(os.Stdout)
		}
		err = f.Render(file)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateInterfaces(c *cli.Context, f *jen.File) error {
	parsed, err := structparser.ParseDirectoryWithFilter(c.String("input"), nil)
	if err != nil {
		return err
	}

	if c.Bool("verbose") {
		pretty, _ := json.MarshalIndent(parsed, "", "\t")
		log.Println("parsed structs", string(pretty))
	}

	{

		generatedInterfaces := []string{}
		for _, strct := range parsed {
			if !strings.HasSuffix(strct.Name, "Repository") {
				continue
			}
			metadata := structMetadata{}
			if len(strct.Docs) == 1 {
				if err := json.Unmarshal([]byte(strct.Docs[0]), &metadata); err != nil {
					return err
				}
			}

			// Interface
			{
				interfaceName := makeFirstUpperCase(strct.Name)
				generatedInterfaces = append(generatedInterfaces, interfaceName)
				f.Type().Id(interfaceName).InterfaceFunc(func(g *Group) {
					for _, v := range strct.Methods {
						g.Id(v.Signature)
					}
				})
			}

		}

		f.Type().Id("Repository").StructFunc(func(g *Group) {
			for _, v := range generatedInterfaces {
				g.Id(strings.TrimSuffix(v, "Repository")).Id(v)
			}
		})

		f.Func().Id("NewRepository").Params(Id("db").Op("*").Qual("go.mongodb.org/mongo-driver/mongo", "Database")).Id("*Repository").BlockFunc(func(g *Group) {
			dict := make(Dict, 0)
			for _, v := range generatedInterfaces {
				dict[Id(strings.TrimSuffix(v, "Repository"))] = Id("New" + v).Call(Id("db"))

			}
			g.Return(Op("&").Id("Repository").Values(dict))
		})
	}
	return nil
}

func generateIndexes(f *File, strct structparser.Struct, meta structMetadata, tagToFieldMap map[string]structparser.Field, structReceiver *Statement) {
	alreadyGenerated := make(map[string]struct{})
	for _, indexDef := range meta.Indexes {
		fields := make([]structparser.Field, 0, len(indexDef.Keys))
		for k := range indexDef.Keys {
			f := tagToFieldMap[k]
			fields = append(fields, f)
		}
		if indexDef.Options.Unique {
			f.Func().Params(structReceiver).Id("GetBy" + getNamesForFunction(fields)).Call(getParams(fields)...).Parens(List(Op("*").Id(strct.Name), Error())).BlockFunc(func(g *Group) {
				g.Id("res").Op(":=").Id(strct.Name).Values()
				g.Err().Op(":=").Id(receiverId).Dot("getCollection").Call().Dot("FindOne").Call(Id("ctx"), Qual("go.mongodb.org/mongo-driver/bson", "D").Values(fieldsToBson(fields)...)).Dot("Decode").Call(Op("&").Id("res"))
				isErrorNoDocuments(g)
				g.Return(Op("&").Id("res"), Nil())
			})
		} else {
			tmpFields := make([]structparser.Field, 0, len(fields))
			for _, v := range fields {
				tmpFields = append(tmpFields, v)
				namesForFunction := getNamesForFunction(tmpFields)
				if _, ok := alreadyGenerated[namesForFunction]; !ok {
					alreadyGenerated[namesForFunction] = struct{}{}
					f.Func().Params(structReceiver).Id("ListBy" + namesForFunction).Call(getParams(tmpFields)...).Parens(List(Index().Id(strct.Name), Error())).BlockFunc(func(g *Group) {
						g.List(Id("cur"), Err()).Op(":=").Id(receiverId).Dot("getCollection").Call().Dot("Find").Call(Id("ctx"), Qual("go.mongodb.org/mongo-driver/bson", "D").Values(fieldsToBson(tmpFields)...))
						isErrorReturnNilErr(g)
						g.Id("res").Op(":=").Index().Id(strct.Name).Values()
						g.If(Err().Op(":=").Id("cur").Dot("All").Call(Id("ctx"), Op("&").Id("res")).Op(";").Id("err").Op("!=").Nil()).Block(
							Return(Nil(), Err()),
						)
						g.Return(Id("res"), Nil())
					})
					f.Func().Params(structReceiver).Id("PageBy" + namesForFunction).Call(append(getParams(tmpFields), Id("page").Id("int64"), Id("limit").Id("int64"))...).Parens(List(Index().Id(strct.Name), Id("*mongopagination.PaginationData"), Id("error"))).BlockFunc(func(g *Group) {
						g.Id("page").Op("+=").Id("1")

						g.Id("res").Op(":=").Index().Id(strct.Name).Values()

						g.Id("filter").Op(":=").Qual("go.mongodb.org/mongo-driver/bson", "M").Values(fieldsToBsonM(tmpFields))

						g.List(Id("paginatedData"), Err()).Op(":=").Id("mongopagination").Dot("New").Call(Id("x.getCollection()")).Dot("Context").Call(Id("ctx")).Dot("Limit").Call(Id("limit")).Dot("Page").Call(Id("page")).Dot("Filter").Call(Id("filter")).Dot("Decode").Call(Id("&res")).Dot("Find").Call()
						g.If(Err().Op("!=").Nil()).Block(
							Return(Nil(), Nil(), Err()),
						)
						g.Return(Id("res"), Id("getPaginationData").Call(Id("paginatedData")), Nil())
						// isErrorReturnNilErr(g)
						// g.Id("res").Op(":=").Index().Id(strct.Name).Values()
						// g.If(Err().Op(":=").Id("cur").Dot("All").Call(Id("ctx"), Op("&").Id("res")).Op(";").Id("err").Op("!=").Nil()).Block(
						// 	Return(Nil(), Err()),
						// )
						// g.Return(Id("res"), Nil())
					})
				}
			}
		}
	}
}

func getParams(fields []structparser.Field) []Code {
	codes := []Code{}
	codes = append(codes, Id("ctx").Qual("context", "Context"))
	for _, v := range fields {
		codes = append(codes, Id(getVarNameForField(v)).Id(v.Type))
	}
	return codes
}

func getNamesForFunction(fields []structparser.Field) string {
	tmp := make([]string, 0, len(fields))
	for _, v := range fields {
		tmp = append(tmp, v.Name)
	}
	return strings.Join(tmp, "And")
}

func getVarNameForField(field structparser.Field) string {
	if field.Name == "Id" || field.Name == "ID" {
		return "id"
	}
	return makeFirstLowerCase(field.Name)
}

func makeFirstLowerCase(s string) string {
	if len(s) < 2 {
		return strings.ToLower(s)
	}

	bts := []byte(s)

	lc := bytes.ToLower([]byte{bts[0]})
	rest := bts[1:]

	return string(bytes.Join([][]byte{lc, rest}, nil))
}

func makeFirstUpperCase(s string) string {
	if len(s) < 2 {
		return strings.ToUpper(s)
	}

	bts := []byte(s)

	lc := bytes.ToUpper([]byte{bts[0]})
	rest := bts[1:]

	return string(bytes.Join([][]byte{lc, rest}, nil))
}

func isErrorNoDocuments(g *Group) *Statement {
	return isError(
		g,
		If(Err().Op("==").Qual("go.mongodb.org/mongo-driver/mongo", "ErrNoDocuments")).Block(
			Return(Nil(), Nil()),
		),
		Return(Nil(), Err()),
	)
}

func isError(g *Group, codes ...Code) *jen.Statement {
	return g.If(Err().Op("!=").Nil()).Block(codes...)
}

func isErrorReturnNilErr(g *Group) *jen.Statement {
	return g.If(Err().Op("!=").Nil()).Block(Return(Nil(), Err()))
}

func fieldToBson(field structparser.Field, value *jen.Statement) *jen.Statement {
	return Values(List(
		Lit(getBsonNameFromField(field)), value,
	))
}

func fieldsToBson(fields []structparser.Field) []jen.Code {
	codes := []Code{}
	for _, field := range fields {
		codes = append(codes, Values((Lit(getBsonNameFromField(field))), Id(getVarNameForField(field))))
	}
	return codes
}

func fieldsToBsonM(fields []structparser.Field) Dict {
	// codes := []Code{}
	dict := Dict{}
	for _, field := range fields {
		dict[Lit(getBsonNameFromField(field))] = Id(getVarNameForField(field))
	}
	return dict
}

func getBsonNameFromField(field structparser.Field) string {
	val, err := structtag.Parse(field.Tag)
	if err != nil {
		panic(err)
	}
	val2, err := val.Get("bson")
	if err != nil {
		panic(err)
	}
	return val2.Name
}

type structMetadata struct {
	CollectionName string            `json:"collection_name"`
	Indexes        []indexDefinition `json:"indexes"`
}

type indexDefinition struct {
	Keys    map[string]int `json:"keys"`
	Options struct {
		Unique bool `json:"unique"`
	}
}
