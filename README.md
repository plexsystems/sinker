# imagesync

`imagesync` enables the syncing of container images from one container registry to another. This is useful in cases where you need to mirror images that exist in a public container registry to a private one.

Images are discovered from Kubernetes resources.

```
NOTE: This project is currently a work in progress. 
Feedback, feature requests, and contributions are welcome!
```

## Installation

`GO111MODULE=on go get github.com/plexsystems/imagesync`

## Usage

The `--mirror` flag tells `imagesync` the host, and optionally a repository path, of the mirror.

For example, given an `images.txt` of the following:

```text
foourl.com/bar/nginxdemos/hello:0.2
foourl.com/bar/alpine:3.11
foourl.com/bar/coreos/prometheus-operator:v0.39.0
```

Running the command:

```console
$ imagesync sync images.txt --mirror foourl.com/bar
```

Would remove `foourl.com/bar` from the images listed above and pull from `docker.io` behind the scenes.

**NOTE:** Given that images are not always sourced from docker.io, some assumptions are made. Most notably, the `coreos/` repository will pull from `quay.io`

This tool assumes that your images use the exact same repository path after the prefix. i.e. Assuming a `--mirror` value of `foourl.com/bar`:

- `foourl.com/bar/nginxdemos/hello:0.2` will be sourced from `docker.io/nginxdemos/hello:0.2`

- `foourl.com/bar/coreos/prometheus-operator:v0.39.0` will be sourced from `quay.io/coreos/prometheus-operator:v0.39.0`

If no `--mirror` flag is used, the images will be read as is.

### Listing images

Print all image references found in a given folder or file. The `--output` flag writes the list to a file.

```console
$ imagesync list manifestsPath --output images.txt
```

### Checking images

Checks if there are any newly published tags for the images.

```console
$ imagesync check images.txt --mirror foourl.com/bar/repo
```

### Syncing images

Sync all images in the image list to the mirror repository.

```console
$ imagesync sync images.txt --mirror foourl.com/bar/repo
```