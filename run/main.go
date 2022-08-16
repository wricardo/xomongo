package main

import (
	"context"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/wricardo/xomongo/example"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	uri := "mongodb://aeon-hire-service:aeon-hire-service@localhost:27017/aeon-hire-service"
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		panic(err)
	}

	repo := example.NewUserRepository(client.Database("aeon-hire-service"))
	u := &example.User{
		Name:      "wallace",
		ShoeSize:  10,
		CreatedAt: time.Time{},
		Profile: example.Profile{
			FavoriteColor: "blue",
		},
	}
	err = repo.Insert(ctx, u)
	if err != nil {
		panic(err)
	}

	fu, err := repo.Get(ctx, u.ID.Hex())
	if err != nil {
		panic(err)
	}
	spew.Dump(`fu: %#v\n`, fu)
	res, err := repo.List(ctx)
	if err != nil {
		panic(err)
	}
	spew.Dump(`: %#v\n`, res)

	fu, err = repo.GetByNameAndShoeSize(ctx, "wallace", 10)
	if err != nil {
		panic(err)
	}
	spew.Dump(`fu: %#v\n`, fu)

}
