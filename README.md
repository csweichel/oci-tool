# oci-tool
Handy little CLI for interacting with OCI data

# Installation
```bash
go install github.com/csweichel/oci-tool@latest
```

I use Gitpod for developing this tool; you should, too :)

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/csweichel/oci-tool)

# Common Tasks

> Note: using a `$ref` is by no means a requirement - it's just really handy for the examples.

## Fetching raw data

When exploring a registry's content, starting with some raw data is handy.

```bash
export ref=docker.io/library/alpine:latest

# download the manifest or manifest list of a reference
oci-tool fetch raw $ref

# download a concrete platform manifest by interpreting the manifest list
export digest=$(oci-tool fetch raw docker.io/library/alpine:latest | jq -r .manifests[0].digest)
oci-tool fetch --digest $digest raw $ref

# download OCI image metadata (see next section for a quicker way of doing this)
export cfgdigest=$(oci-tool fetch --digest $digest raw $ref | jq .config.digest)
oci-tool fetch --digest $cfgdigest raw $ref
```

## Fetching an image manifest, image and config

> Note: If ref points to an index rather than a manifest, you can also use `--digest` instead of `--platform` and point to a specific manifest directly.

```bash
export ref=docker.io/library/alpine:latest

# fetch the image manifest (in case of alpine, it's an index instead of a manifest)
oci-tool fetch manifest $ref

# choose an actual manifest, e.g. the linux on amd64
oci-tool fetch manifest --platform=linux-amd64 $ref

# fetch the image config
oci-tool fetch image --platform=linux-amd64 $ref
```

## Fetching a file from a layer

```
export ref=docker.io/library/alpine:latest

# download busybox from alpine
oci-tool fetch file --platform=linux-amd64 $ref bin/busybox

# list all files in the layers
oci-tool --verbose fetch file --platform=linux-amd64 $ref does-not-exist
```

## Inspecting image layer

> Note: If ref points to an index rather than a manifest, you can also use `--digest` instead of `--platform` and point to a specific manifest directly.

```bash
export ref=docker.io/library/alpine:latest

# print the total downloadable layer size (sum of all layers)
oci-tool layer --platform=linux-amd64 size $ref

# list all layers before unpacking
oci-tool layer --platform=linux-amd64 list $ref

# list all layer digests after unpacking
oci-tool layer --platform=linux-amd64 list --unpacked $ref
```


## Resolving references

```bash
# resolve alpine:latest to it's non-familiar digested name
oci-tool resolve name alpine:latest

# print the descriptor a reference resolves to
oci-tool resolve descriptor docker.io/library/alpine:latest
```

# Uncommon tasks

With the primitives outlined above you can already to a lot of fun things, especially when combined with [jq](https://stedolan.github.io/jq/) and some bash.

## Finding the layer diff between two images
```bash
oci-tool layer --platform=linux-amd64 list docker.io/library/alpine:3.13 | jq .[].digest | sort > alpine-3.13.txt
oci-tool layer --platform=linux-amd64 list docker.io/library/alpine:3.14 | jq .[].digest | sort > alpine-3.14.txt

diff alpine-3.13.txt alpine-3.14.txt
```

## Inspect a Helm chart stored in a registry
Helm 3 can save [charts in an OCI registry](https://helm.sh/docs/topics/registries/). If you've ever wondered how those look in the registry, use `oci-tool` to inspect what's actually stored there. 

```bash
# assuming $ref points to a chart

# fetch the manifest
oci-tool fetch manifest $ref

# fetch the helm chart config
export digest=$(oci-tool fetch manifest $ref | jq -r .config.digest)
oci-tool fetch --digest $digest --media-type application/vnd.cncf.helm.config.v1+json raw $ref
```
