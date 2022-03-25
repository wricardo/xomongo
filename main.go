package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "aeon-ci",
		Usage: "aeon-ci",
		Flags: []cli.Flag{},
		Commands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "generate --input [file_or_directory] --out [file.go]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "input",
						Usage:    "file or directory",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "output",
						Usage:    "example: file.go",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "package",
						Usage:    "package name for the generated file",
						Required: true,
					},
					&cli.BoolFlag{
						Name: "verbose",
					},
				},
				Action: generate,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
