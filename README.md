# Sinker

[![Go Report Card](https://goreportcard.com/badge/github.com/plexsystems/sinker)](https://goreportcard.com/report/github.com/plexsystems/sinker)
[![GitHub release](https://img.shields.io/github/release/plexsystems/sinker.svg)](https://github.com/plexsystems/sinker/releases)

![logo](assets/logo.png)

`sinker` syncs container images from one registry to another. This is useful in cases when you rely on images that exist in a public container registry, but need to pull from a private registry.

Images can be sync'd either by using [The image manifest](#the-image-manifest) or via the command line.

See the [example](https://github.com/plexsystems/sinker/tree/main/example) folder for more details on the produced files.

## Installation

`GO111MODULE=on go get github.com/plexsystems/sinker`

Releases are also provided in the [releases](https://github.com/plexsystems/sinker/releases) tab on GitHub.

## The image manifest

### The target section

```yaml
target:
  host: mycompany.com
  repository: myteam
```

The `target` section is where the images will be synced to. The above yaml would sync all images to the `myteam` repository hosted at `mycompany.com` (`mycompany.com/myteam/...`)

### The images section

```yaml
target:
  host: mycompany.com
  repository: myteam
sources:
- repository: coreos/prometheus-operator
  host: quay.io
  tag: v0.40.0
- repository: super/secret
  tag: v0.3.0
  auth:
    username: DOCKER_USER_ENV
    password: DOCKER_PASSWORD_ENV
- repository: nginx
  digest: sha256:bbda10abb0b7dc57cfaab5d70ae55bd5aedfa3271686bace9818bba84cd22c29
```

### Optional host defaults to Docker Hub

In both the `target` and `sources` section, the `host` field is _optional_. When no host is set, the host is assumed to be Docker Hub.

### Auth

All auth is handled by looking at the clients Docker auth. If the client can perform a `docker push` or `docker pull`, sinker will be able to as well.

Optionally, the `auth` section allows you to set the names of _environment variables_ that will be used for creating basic auth to the registry. This could be useful in pipelines where auth is stored in environment variables.

## Sync behavior

If the `target` registry supports nested paths, the entire source repository will be pushed to the target. For example, the `prometheus-operator` would be pushed to:

```text
mycompany.com/myteam/coreos/prometheus-operator:v0.40.0
```

**Registries that support nested paths:** Azure Container Registry (ACR), Amazon Elastic Container Registry (ECR), Google Container Registry (GCR)

If the `target` registry does _not_ support nested paths, only the base path of the source will be pushed to the target registry. For example, the `prometheus-operator` would be pushed to:

```text
mycompany.com/myteam/prometheus-operator:v0.40.0
```

**Registries that do not support nested paths:** Docker Hub, Quay.io

## Usage

Descriptions of commands and flags to help understand how to use Sinker.

![demo](assets/sinker-pull-demo.gif)

### Global flags

#### --manifest

Set the directory or location of the manifest file to read from. Defaults to `.images.yaml` in the working directory.

### Push command

Push all of the images inside of the image manifest to the target registry.

```shell
$ sinker push
```

#### --dryrun flag (optional)

The `--dryrun` flag will print out a summary of the images that do not exist at the target registry and the fully qualified names of the images that will be pushed.

#### --images and --target flags (optional)

A list of images can be specified with the `--images` flag. Set the target with `--target` (required).

```shell
$ sinker push -i busybox:latest,quay.io/coreos/prometheus-operator:v0.40.0 -t host.com/repo
```

### Pull command

Pulls the source or target images found in the image manifest.

Pulling the `source` could be useful if you want to perform additional actions on the image(s) before performing a push operation (e.g. scanning for vulnerabilities).

Pulling the `target` could be useful if you need to load the images into another environment, such as [Kind](https://github.com/kubernetes-sigs/kind).

```shell
$ sinker pull <source|target>
```

#### --images flag (optional)

A list of images to pull, delimeted by commas.

### List command

Prints a list of either the `source` or `target` images that exist in the image manifest. This can be useful for piping into additional tooling that acts on image urls.

```shell
$ sinker list <source|target>
```

#### --output flag (optional)

Outputs the list to a file (e.g. `source-images.txt`).

### Check command

Checks if any of the source images found in the image manifest have new updates.

```shell
$ sinker check <standard input>
```

#### --images flag (optional)

A list of images to check updates for, delimeted by commas.

### Create command

Create an image manifest that will sync images to the given target registry.

```shell
$ sinker create <file|directory|standard input> --target mycompany.com/myteam
```

#### --target flag (required)

Specifies the target registry (and optionally a repository) to sync the images to.

#### --output flag (optional)

Outputs the updated manifest to the specified path.

- If a directory is specified, the manifest will be created in the directory with the name `.images.yaml`.

- If a path is a yaml file, the manifest will be created at the given path.

#### Passing in a search critera

Find all image references in the search criteria (file, directory, etc)

While this tool is not Kubernetes specific, currently the `create` and `update` commands can take a file or directory to find all Kubernetes manifests and extract the image references from them. This includes images specified in container arguments as well as CRDs such as `Prometheus` and `Alertmanager`.

The intent is that this can be expanded to support other workloads (e.g docker compose).

```shell
$ sinker create example/bundle.yaml --target mycompany.com/myteam
```

```yaml
target:
  host: mycompany.com
  repository: myteam
sources:
- repository: coreos/prometheus-operator
  host: quay.io
  tag: v0.40.0
- repository: jimmidyson/configmap-reload
  tag: v0.3.0
- repository: coreos/prometheus-config-reloader
  host: quay.io
  tag: v0.40.0
```

Additionally, standard input is accepted. This is useful for creating and updating manifests based on images found in currently active Kubernetes clusters.

```shell
$ kubectl get pods --all-namespaces -o jsonpath="{.items[*].spec.containers[*].image}" | ./sinker create - --target repo
```

```shell
$ kubectl get pods --all-namespaces -o jsonpath="{.items[*].spec.containers[*].image}" | ./sinker update -
```

### Update command

Updates the current image manifest to reflect new changes found in the Kubernetes manifest(s).

```shell
$ sinker update <file|directory|standard input>
```

#### --output flag (optional)

Outputs the updated manifest to the specified path.

- If a directory is specified, the manifest will be created in the directory with the name `.images.yaml`.

- If a path is a yaml file, the manifest will be created at the given path.

_NOTE: The update command will ONLY update image **versions**. This allows for pinning of certain fields you want to manage yourself (source registry, auth)._
