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

### Listing images

Print all image references found in a given folder or file. The `--output` flag writes the list to a file.

```console
$ imagesync list manifests --output images.txt
```

### Checking images

Find all image references and checks if there are any newly published tags for the images.

```console
$ imagesync check manifests
```

### Syncing images

Sync all images from source resources to destination resources. The source resources are needed to know which container registry the images are hosted at.

```console
$ imagesync sync manifests kustomizedmanifests
```