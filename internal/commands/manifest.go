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
	Target Target           `yaml:"target"`
	Images []ContainerImage `yaml:"images,omitempty"`
}

// Target is the registry and repository to sync images to
type Target struct {
	Registry   string `yaml:"registry"`
	Repository string `yaml:"repository,omitempty"`
}

func (t Target) String() string {
	if t.Repository == "" {
		return t.Registry
	}

	return t.Registry + "/" + t.Repository
}

// ContainerImage is a container image
type ContainerImage struct {
	Repository     string `yaml:"repository"`
	Version        string `yaml:"version"`
	SourceRegistry string `yaml:"source,omitempty"`
	Auth           Auth   `yaml:"auth,omitempty"`
}

// Source returns the source image
func (c ContainerImage) Source() string {
	if c.SourceRegistry == "docker.io" && !strings.Contains(c.Repository, "/") {
		return c.SourceRegistry + "/library/" + c.Repository + ":" + c.Version
	}

	if c.SourceRegistry == "" {
		return c.Repository + ":" + c.Version
	}

	return c.SourceRegistry + "/" + c.Repository + ":" + c.Version
}

// Target returns the target image
func (c ContainerImage) Target(target Target) string {
	return target.String() + "/" + c.Repository + ":" + c.Version
}

// Auth is a username and password to log into a registry
type Auth struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// NewManifest returns a new image manifest
func NewManifest(target string) Manifest {
	var registry string
	var repository string

	targetTokens := strings.Split(target, "/")
	if len(targetTokens) > 1 {
		registry = targetTokens[0]
		repository = strings.Join(targetTokens[1:], "/")
	} else {
		registry = target
	}

	targetLocation := Target{
		Registry:   registry,
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
