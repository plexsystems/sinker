package commands

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

const manifestFileName = ".images.yaml"

// Manifest is a collection of images to sync
type Manifest struct {
	Target Registry         `yaml:"target"`
	Images []ContainerImage `yaml:"images,omitempty"`
}

// Registry is the registry and repository to sync images to
type Registry struct {
	Host       string `yaml:"host"`
	Repository string `yaml:"repository,omitempty"`
	Auth       Auth   `yaml:"auth,omitempty"`
}

func (r Registry) String() string {
	if r.Repository == "" {
		return r.Host
	}

	return r.Host + "/" + r.Repository
}

// ContainerImage is a container image
type ContainerImage struct {
	Source  Registry `yaml:"source,omitempty"`
	Version string   `yaml:"version"`
	Target  Registry `yaml:"target,omitempty"`
}

// SourceImage returns the source image
func (c ContainerImage) SourceImage() string {
	if c.Source.Host == "docker.io" && !strings.Contains(c.Source.Repository, "/") {
		return c.Source.Host + "/library/" + c.Source.Repository + ":" + c.Version
	}

	if c.Source.Host == "" {
		return c.Source.Repository + ":" + c.Version
	}

	return c.Source.Host + "/" + c.Source.Repository + ":" + c.Version
}

// TargetImage returns the target image
func (c ContainerImage) TargetImage() string {
	return c.Target.String() + "/" + c.Source.Repository + ":" + c.Version
}

// Auth is a username and password to log into a registry
type Auth struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// NewManifest returns a new image manifest
func NewManifest(target string) Manifest {
	var host string
	var repository string

	targetTokens := strings.Split(target, "/")
	if len(targetTokens) > 1 {
		host = targetTokens[0]
		repository = strings.Join(targetTokens[1:], "/")
	} else {
		host = target
	}

	targetLocation := Registry{
		Host:       host,
		Repository: repository,
	}

	manifest := Manifest{
		Target: targetLocation,
	}

	return manifest
}

// NewAutodetectManifest returns a new image manifest with images found in the repository
func NewAutodetectManifest(target string, path string) (Manifest, error) {
	manifest := NewManifest(target)

	foundImages, err := getFromKubernetesManifests(path, manifest.Target)
	if err != nil {
		return Manifest{}, fmt.Errorf("get from kubernetes manifests: %w", err)
	}

	manifest.Images = foundImages

	return manifest, nil
}

// GetManifest returns the current manifest file in the working directory
func GetManifest() (Manifest, error) {
	imageManifestContents, err := ioutil.ReadFile(manifestFileName)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading manifest: %w", err)
	}

	var currentImageManifest Manifest
	if err := yaml.Unmarshal(imageManifestContents, &currentImageManifest); err != nil {
		return Manifest{}, fmt.Errorf("unmarshal current manifest: %w", err)
	}

	for i := range currentImageManifest.Images {
		if currentImageManifest.Images[i].Target.Host == "" {
			currentImageManifest.Images[i].Target = currentImageManifest.Target
		}
	}

	return currentImageManifest, nil
}

// WriteManifest writes the image manifest to disk
func WriteManifest(manifest Manifest) error {
	imageManifestContents, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshal image manifest: %w", err)
	}
	imageManifestContents = bytes.ReplaceAll(imageManifestContents, []byte(`"`), []byte(""))

	if err := ioutil.WriteFile(manifestFileName, imageManifestContents, os.ModePerm); err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	return nil
}
