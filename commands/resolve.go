package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/distribution/reference"
	cli "github.com/urfave/cli/v2"
)

var Resolve = &cli.Command{
	Name:  "resolve",
	Usage: "resolves a reference",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "familiar",
			Value: true,
			Usage: "interpret reference as familiar Docker reference, e.g. alpine:latest as docker.io/library/alpine:latest",
		},
	},
	Subcommands: []*cli.Command{
		{
			Name:      "name",
			Usage:     "resolves a reference to its digested name",
			ArgsUsage: "<ref>",
			Action:    actionResolve(false, "name"),
		},
		{
			Name:      "descriptor",
			Aliases:   []string{"desc"},
			Usage:     "resolves a reference to its descriptor",
			ArgsUsage: "<ref>",
			Action:    actionResolve(true, "descriptor"),
		},
	},
}

func actionResolve(resolveDesc bool, action string) func(c *cli.Context) error {
	return func(c *cli.Context) error {
		ref := fromArgsGetRef(c, action)

		res, err := fromFlagsGetResolver(c)
		if err != nil {
			return err
		}

		ctx, cancel := fromFlagsGetContext(c)
		defer cancel()

		if c.Bool("familiar") {
			pref, err := reference.ParseAnyReference(ref)
			if err != nil {
				return fmt.Errorf("error parsing %v: %w", ref, err)
			}
			ref = pref.String()
		}

		nme, desc, err := res.Resolve(ctx, ref)
		if err != nil {
			return err
		}
		if resolveDesc {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(desc)
		}

		pref, err := reference.ParseNamed(nme)
		if err != nil {
			return err
		}
		cref, err := reference.WithDigest(pref, desc.Digest)
		if err != nil {
			return err
		}

		fmt.Println(cref.String())
		return nil
	}
}
