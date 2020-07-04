# Sinker

`sinker` syncs container images from one registry to another. This is useful in cases where you need to have images that exist in a public container registry, in a private one.

See the [example](https://github.com/plexsystems/sinker/tree/main/example) folder for more details on the produced files.

## Installation

`GO111MODULE=on go get github.com/plexsystems/sinker`

Releases are also provided in the releases tab on GitHub.

## The image manifest

All commands either create, update, or read from the image manifest (`.images.yaml`). 

While the `create` and `update` commands assist with managing the image manifest, the `push` command will not modify the image manifest. Allowing you to manually control the manifest if desired.

### The target section

```yaml
target:
    registry: mycompany.com
    repository: myteam
```

The `target` section is where all of the images will be synced to. The above yaml would sync all images to the `mycompany` registry in the `myteam` repository: `mycompany.com/myteam/...`. 

The `repository` field is optional.

### The images section

```yaml
target:
    registry: mycompany.com
    repository: myteam
images:
  - repository: coreos/prometheus-operator
    version: v0.40.0
    source: quay.io
  - repository: jimmidyson/configmap-reload
    version: v0.3.1
    source: docker.io
  - repository: super/secret
    version: v1.0.0
    source: privatecompany.org
    auth:
        username: USER_ENVIRONMENT_VARIABLE_NAME
        password: PASSWORD_ENVIRONMENT_VARIABLE_NAME
```

The images section includes the source registry and the repository for the image. During a sync, an image `push` would result in `mycompany.com/myteam/coreos/prometheus-operator:v0.40.0`

#### Auth

All auth is handled by looking at the clients Docker auth. If the client can perform a `docker push` or `docker pull`, sinker will be able to as well.

In the event that an image that needs to be sync'd is in another registry, the `auth` section allows you to set the names of _environment variables_ that will be used for creating basic auth to the registry. This is useful in CI pipelines.

## Usage

All examples are taken from running commands in the `examples/` folder.

### Create command

Create an image manifest that includes a target registry and a collection of images that should be synced to the target.

```
$ sinker create --target mycompany.com/myteam
```

#### --target flag (required)

Specifies the target registry (and optionally a repository) to sync the images to

#### --autodetect flag (optional)

While this tool is not Kubernetes specific, currently the `create` and `update` commands find all Kubernetes manifests and extracts the image references from them. This includes images specified in container arguments as well as CRDs such as `Prometheus` and `Alertmanager`.

```
$ sinker create --target mycompany.com/myteam --autodetect
```

```yaml
target:
    registry: mycompany.com
    repository: myteam
images:
  - repository: coreos/prometheus-operator
    version: v0.40.0
    source: quay.io
  - repository: jimmidyson/configmap-reload
    version: v0.3.1
    source: docker.io
  - repository: coreos/prometheus-config-reloader
    version: v0.40.0
    source: quay.io
```

#### Push command

Push all of the images inside of the image manifest to the target registry.

```
$ sinker push
```

#### --dryrun flag (optional)

The `--dryrun` flag will print out a summary of the images that do not exist at the target registry and the fully qualified names of the images that will be pushed.

### Update command

```
$ sinker update
```

Updates the current manifest to reflect new changes to Kubernetes manifests.

_NOTE: The update command will ONLY update image **versions**. This allows for pinning of certain fields you want to manage yourself (source registry, auth)_

#### --target flag (optional)

If desired, you can set a new target in the image manifest by using --target during an update.

### List command

```
$ sinker list target
```

Prints a list of either the `source` or `target` images. This can be useful for piping into additional tooling that acts on image urls.

#### --output flag (optional)

Outputs the list to a file (e.g. `images.txt`)

### Check command

```
$ sinker check
```

Checks if any of the images found in the image manifest have new updates. Currently only works for the source images that are hosted on Docker Hub.
