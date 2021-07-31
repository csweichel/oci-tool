package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/containerd/containerd/remotes"
	"github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli/v2"
)

var Fetch = &cli.Command{
	Name:  "fetch",
	Usage: "Fetches OCI data from a regitry",
	// Action:    actionFetchDirect,
	Subcommands: []*cli.Command{
		{
			Name:      "config",
			Usage:     "fetches the config of an image - assumes ref points to a manifest",
			ArgsUsage: "<ref>",
			Action:    actionFetchConfig,
		},
		{
			Name:      "raw",
			Usage:     "fetches data directly without trying to interpret it",
			ArgsUsage: "<ref>",
			Action:    actionFetchDirect,
		},
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "digest",
			Usage: "digest of the object to fetch",
		},
		&cli.StringFlag{
			Name:  "media-type",
			Usage: "media-type of the object to fetch",
		},
		&cli.BoolFlag{
			Name:  "descriptor-from-stdin",
			Usage: "parses the OCI descriptor of the object to fetch from STDIN. --digest and --media-type override values parsed from STDIN.",
		},
	},
}

func fetchManifest(ctx context.Context, res remotes.Resolver, ref string, dgst digest.Digest) (resolvedName string, mf *ociv1.Manifest, err error) {
	name, desc, err := res.Resolve(ctx, ref)
	if err != nil {
		return "", nil, fmt.Errorf("cannot resolve %v: %w", ref, err)
	}
	if dgst != "" {
		desc.Digest = dgst
	}
	fetcher, err := res.Fetcher(ctx, name)
	if err != nil {
		return "", nil, err
	}
	mfin, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return "", nil, err
	}
	defer mfin.Close()

	var mfo ociv1.Manifest
	err = json.NewDecoder(mfin).Decode(&mfo)
	if err != nil {
		return "", nil, fmt.Errorf("cannot decode manifest: %w", err)
	}

	return name, &mfo, nil
}

func fetchAndUnmarshal(ctx context.Context, res remotes.Resolver, ref string, desc ociv1.Descriptor, obj interface{}) error {
	fetcher, err := res.Fetcher(ctx, ref)
	if err != nil {
		return err
	}

	in, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return err
	}
	defer in.Close()

	dec := json.NewDecoder(in)
	return dec.Decode(obj)
}

func fromArgsGetRef(c *cli.Context, cmd string) (ref string) {
	ref = c.Args().Get(0)
	if ref == "" {
		fmt.Println("missing ref")
		cli.ShowCommandHelpAndExit(c, cmd, 1)
	}
	return
}

func fromFlagsGetDigest(c *cli.Context) (digest.Digest, error) {
	dgst := c.String("digest")
	if dgst == "" {
		return "", nil
	}

	res, err := digest.Parse(dgst)
	if err != nil {
		return "", fmt.Errorf("cannot parse digest %v: %w", dgst, err)
	}
	return res, nil
}

func actionFetchConfig(c *cli.Context) error {
	ref := fromArgsGetRef(c, "config")

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

	name, mf, err := fetchManifest(ctx, res, ref, digest)
	if err != nil {
		return err
	}

	fetcher, err := res.Fetcher(ctx, name)
	if err != nil {
		return err
	}

	cfgin, err := fetcher.Fetch(ctx, mf.Config)
	if err != nil {
		return err
	}
	defer cfgin.Close()

	_, err = io.Copy(os.Stdout, cfgin)
	if err != nil {
		return err
	}

	return nil
}

func actionFetchDirect(c *cli.Context) error {
	ref := fromArgsGetRef(c, "raw")

	digest, err := fromFlagsGetDigest(c)
	if err != nil {
		return err
	}
	var customDesc ociv1.Descriptor
	if c.Bool("descriptor-from-stdin") {
		dec := json.NewDecoder(os.Stdin)
		dec.DisallowUnknownFields()
		err := dec.Decode(&customDesc)
		if err != nil {
			return fmt.Errorf("descriptor parse error: %w", err)
		}
	}

	res, err := fromFlagsGetResolver(c)
	if err != nil {
		return err
	}

	ctx, cancel := fromFlagsGetContext(c)
	defer cancel()

	name, desc, err := res.Resolve(ctx, ref)
	if err != nil {
		return fmt.Errorf("cannot resolve %v: %w", ref, err)
	}
	if customDesc.Digest != "" {
		desc = customDesc
	}

	fetcher, err := res.Fetcher(ctx, name)
	if err != nil {
		return err
	}
	if digest != "" {
		desc.Digest = digest
	}
	if ct := c.String("media-type"); ct != "" {
		desc.MediaType = ct
	}
	fres, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return err
	}
	defer fres.Close()

	_, err = io.Copy(os.Stdout, fres)
	if err != nil {
		return err
	}

	return nil
}
