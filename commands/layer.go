package commands

import (
	"encoding/json"
	"fmt"
	"os"

	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	cli "github.com/urfave/cli/v2"
)

var Layer = &cli.Command{
	Name:  "layer",
	Usage: "provides information about an image's layers - assumes ref and digest point to a manifest",
	Subcommands: []*cli.Command{
		{
			Name:      "list",
			Usage:     "lists the layer digests",
			ArgsUsage: "<ref>",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "unpacked",
					Usage: "list digests of the unpacked layer (from the image config)",
				},
			},
			Action: actionListLayer,
		},
		{
			Name:      "size",
			Usage:     "prints the total bytes of all downloaded layers",
			ArgsUsage: "<ref>",
			Action:    actionSizeLayer,
		},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "digest",
			Usage: "digest of the object where to start from",
		},
	},
}

func actionListLayer(c *cli.Context) error {
	ref := fromArgsGetRef(c, "list")
	res, err := fromFlagsGetResolver(c)
	if err != nil {
		return err
	}

	ctx, cancel := fromFlagsGetContext(c)
	defer cancel()

	digest, err := fromFlagsGetDigest(c)
	if err != nil {
		return err
	}

	resolved, mf, err := fetchManifest(ctx, res, ref, digest)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	if !c.Bool("unpacked") {
		return enc.Encode(mf.Layers)
	}

	var cfg ociv1.Image
	err = fetchAndUnmarshal(ctx, res, resolved, mf.Config, &cfg)
	if err != nil {
		return fmt.Errorf("cannot fetch image: %w", err)
	}

	return enc.Encode(cfg.RootFS)
}

func actionSizeLayer(c *cli.Context) error {
	ref := fromArgsGetRef(c, "list")
	res, err := fromFlagsGetResolver(c)
	if err != nil {
		return err
	}

	ctx, cancel := fromFlagsGetContext(c)
	defer cancel()

	digest, err := fromFlagsGetDigest(c)
	if err != nil {
		return err
	}

	_, mf, err := fetchManifest(ctx, res, ref, digest)
	if err != nil {
		return err
	}

	var size int64
	for _, l := range mf.Layers {
		size += l.Size
	}

	fmt.Println(size)
	return nil
}
