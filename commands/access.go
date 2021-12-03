package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/cli/cli/config/configfile"
	cli "github.com/urfave/cli/v2"
)

// fromFlagsGetResolver returns a configured containerd resolver
func fromFlagsGetResolver(c *cli.Context) (res remotes.Resolver, err error) {
	var authRegOpt docker.RegistryOpt
	if f := c.String("docker-config"); f != "" {
		authRegOpt, err = getAuthorizerFromDockerConfig(f)
		if err != nil {
			return nil, err
		}
	}
	auth := docker.ConfigureDefaultRegistries(authRegOpt)

	insecure := c.Bool("insecure")

	var resolverOpts docker.ResolverOptions
	resolverOpts.Hosts = func(s string) (res []docker.RegistryHost, err error) {
		if auth != nil {
			res, err = auth(s)
			if err != nil {
				return nil, err
			}
		}

		if insecure {
			if len(res) == 0 {
				res = []docker.RegistryHost{{Scheme: "http"}}
			} else {
				for i := range res {
					res[i].Scheme = "http"
				}
			}
		}

		return
	}

	return docker.NewResolver(resolverOpts), nil
}

func getAuthorizerFromDockerConfig(fn string) (docker.RegistryOpt, error) {
	// authorizerFromDockerConfig turns docker client config into docker registry hosts
	authorizerFromDockerConfig := func(cfg *configfile.ConfigFile) docker.Authorizer {
		return docker.NewDockerAuthorizer(docker.WithAuthCreds(func(host string) (user, pass string, err error) {
			auth, err := cfg.GetAuthConfig(host)
			if err != nil {
				return
			}
			user = auth.Username
			pass = auth.Password
			return
		}))
	}

	var mayNotExist bool
	if strings.HasPrefix(fn, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		fn = filepath.Join(home, strings.TrimPrefix(fn, "~/"))
		mayNotExist = true
	}

	fr, err := os.OpenFile(fn, os.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) && mayNotExist {
			return nil, nil
		}

		return nil, fmt.Errorf("open(%s): %w", fn, err)
	}
	defer fr.Close()

	dockerCfg := configfile.New(fn)
	err = dockerCfg.LoadFromReader(fr)
	if err != nil {
		return nil, err
	}

	return docker.WithAuthorizer(authorizerFromDockerConfig(dockerCfg)), nil
}

// fromFlagsGetContext produces a context that times out as configured using the flags
func fromFlagsGetContext(c *cli.Context) (ctx context.Context, cancel context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.Duration("timeout"))
}
