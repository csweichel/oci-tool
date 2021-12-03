package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/containerd/containerd/remotes"
	"github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
)

var Fetch = &cli.Command{
	Name:  "fetch",
	Usage: "Fetches OCI data from a regitry",
	// Action:    actionFetchDirect,
	Subcommands: []*cli.Command{
		{
			Name:      "image",
			Usage:     "fetches an image's metadata - assumes ref points to a manifest",
			ArgsUsage: "<ref>",
			Action:    actionFetchImageMD,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "platform",
					Usage: "select a particular platform",
				},
			},
		},
		{
			Name:      "manifest",
			Usage:     "fetches an image manifest",
			ArgsUsage: "<ref>",
			Action:    actionFetchManifest,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "platform",
					Usage: "select a particular platform",
				},
			},
		},
		{
			Name:      "raw",
			Usage:     "fetches data directly without trying to interpret it",
			ArgsUsage: "<ref>",
			Action:    actionFetchDirect,
		},
		{
			Name:      "layer",
			Usage:     "downloads a layer blob",
			ArgsUsage: "<ref>",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "position",
					Aliases:  []string{"p"},
					Usage:    `Either "first", "last", a positive number indexing layers starting from the first or a negative number indexing layers starting from the last`,
					Required: true,
				},
				&cli.StringFlag{
					Name:        "output",
					Aliases:     []string{"o"},
					TakesFile:   true,
					DefaultText: "<layer-hash>.tar.gz",
					Usage:       "output path",
				},
				&cli.StringFlag{
					Name:  "platform",
					Usage: "select a particular platform",
				},
				&cli.BoolFlag{
					Name:    "quiet",
					Aliases: []string{"q"},
				},
			},
			Action: actionFetchLayer,
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
		&cli.BoolFlag{
			Name:  "insecure",
			Usage: "fetch via HTTP rather than HTTPS",
		},
	},
}

func actionFetchManifest(c *cli.Context) error {
	ref := fromArgsGetRef(c, "manifest")

	dgst, err := fromFlagsGetDigest(c)
	if err != nil {
		return err
	}

	res, err := fromFlagsGetResolver(c)
	if err != nil {
		return err
	}

	ctx, cancel := fromFlagsGetContext(c)
	defer cancel()

	plt := c.String("platform")

	_, mf, err := interactiveFetchManifestOrIndex(ctx, res, ref, plt, dgst)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(mf)
}

func interactiveFetchManifestOrIndex(ctx context.Context, res remotes.Resolver, ref, plt string, dgst digest.Digest) (name string, result *ociv1.Manifest, err error) {
	resolved, desc, err := res.Resolve(ctx, ref)
	if err != nil {
		return "", nil, fmt.Errorf("cannot resolve %v: %w", ref, err)
	}

	if dgst != "" {
		desc.Digest = dgst
	}

	fetcher, err := res.Fetcher(ctx, resolved)
	if err != nil {
		return "", nil, err
	}

	in, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return "", nil, err
	}
	defer in.Close()
	buf, err := ioutil.ReadAll(in)
	if err != nil {
		return "", nil, err
	}

	var mf ociv1.Manifest
	err = json.Unmarshal(buf, &mf)
	if err != nil {
		return "", nil, fmt.Errorf("cannot unmarshal manifest: %w", err)
	}

	if mf.Config.Size != 0 {
		return resolved, &mf, nil
	}

	var mfl ociv1.Index
	err = json.Unmarshal(buf, &mfl)
	if err != nil {
		return "", nil, err
	}

	if plt != "" {
		var dgst digest.Digest
		for _, mf := range mfl.Manifests {
			if fmt.Sprintf("%s-%s", mf.Platform.OS, mf.Platform.Architecture) == plt {
				dgst = mf.Digest
			}
		}
		if dgst == "" {
			return "", nil, fmt.Errorf("no manifest for platform %s found", plt)
		}

		fmt.Fprintf(os.Stderr, "found manifest for %s: %s\n", plt, dgst)

		var mf *ociv1.Manifest
		_, mf, err = fetchManifest(ctx, res, resolved, dgst)
		if err != nil {
			return "", nil, err
		}

		return resolved, mf, nil
	}

	fmt.Fprintf(os.Stderr, "%s points to an index rather than a manifest.\n", ref)
	fmt.Fprintf(os.Stderr, "Use --platform to select a manifest. Possible choices are:\n")
	for _, mf := range mfl.Manifests {
		fmt.Fprintf(os.Stderr, "\t%s-%s\n", mf.Platform.OS, mf.Platform.Architecture)
	}

	os.Exit(2)
	return "", nil, nil
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

func actionFetchImageMD(c *cli.Context) error {
	ref := fromArgsGetRef(c, "config")

	res, err := fromFlagsGetResolver(c)
	if err != nil {
		return err
	}

	ctx, cancel := fromFlagsGetContext(c)
	defer cancel()

	dgst, err := fromFlagsGetDigest(c)
	if err != nil {
		return err
	}

	plt := c.String("platform")

	name, mf, err := interactiveFetchManifestOrIndex(ctx, res, ref, plt, dgst)
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

	var ctn map[string]interface{}

	err = json.NewDecoder(cfgin).Decode(&ctn)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(ctn)
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

func actionFetchLayer(c *cli.Context) error {
	pos := c.String("position")

	ref := fromArgsGetRef(c, "manifest")

	dgst, err := fromFlagsGetDigest(c)
	if err != nil {
		return err
	}

	res, err := fromFlagsGetResolver(c)
	if err != nil {
		return err
	}

	ctx, cancel := fromFlagsGetContext(c)
	defer cancel()

	plt := c.String("platform")

	_, mf, err := interactiveFetchManifestOrIndex(ctx, res, ref, plt, dgst)
	if err != nil {
		return err
	}
	if len(mf.Layers) == 0 {
		return fmt.Errorf("manifest has no layer")
	}

	var (
		layer ociv1.Descriptor
		lc    = len(mf.Layers)
	)
	switch pos {
	case "last":
		layer = mf.Layers[lc-1]
	case "first":
		layer = mf.Layers[0]
	default:
		pos, err := strconv.Atoi(pos)
		if err != nil {
			return err
		}
		posAbs := pos
		if posAbs < 0 {
			posAbs *= -1
		}
		if posAbs > lc {
			return fmt.Errorf("position exceeds number of layers: %d > %d", posAbs, lc)
		}

		if pos < 0 {
			layer = mf.Layers[lc-(posAbs+1)]
		} else {
			layer = mf.Layers[posAbs]
		}
	}

	fetcher, err := res.Fetcher(ctx, ref)
	if err != nil {
		return err
	}

	dl, err := fetcher.Fetch(ctx, layer)
	if err != nil {
		return err
	}
	defer dl.Close()

	// TODO: determine file ending based on content type
	fn := c.String("output")
	if fn == "" {
		fn = layer.Digest.Hex() + "tar.gz"
	}
	f, err := os.OpenFile(fn, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	var bar io.Writer
	if c.Bool("quiet") {
		bar = io.Discard
	} else {
		bar = progressbar.DefaultBytes(
			layer.Size,
			"downloading "+layer.Digest.String()+" ",
		)
	}
	_, err = io.Copy(io.MultiWriter(f, bar), dl)
	if err != nil {
		return err
	}

	return nil
}
