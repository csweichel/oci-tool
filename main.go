package main

import (
	"log"
	"os"
	"time"

	"github.com/csweichel/oci-tool/commands"
	"github.com/urfave/cli/v2"
)

var app = &cli.App{
	Name:  "oci-tool",
	Usage: "handy little CLI for interacting with OCI data",
	Commands: []*cli.Command{
		commands.Fetch,
		commands.Layer,
	},
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name:  "docker-config",
			Usage: "path to a Docker config file to use for authentication",
			Value: "~/.docker/config.json",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: "timeout for the entire operation",
			Value: 1 * time.Minute,
		},
	},
	EnableBashCompletion: true,
}

func main() {
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
