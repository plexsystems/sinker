package commands

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/plexsystems/sinker/internal/docker"

	"gopkg.in/yaml.v2"
)

// Manifest is a collection of images to sync
type Manifest struct {
	Target Target        `yaml:"target"`
	Images []SourceImage `yaml:"sources,omitempty"`
}

// Target is a target location for an image
type Target struct {
	Host       string `yaml:"host,omitempty"`
	Repository string `yaml:"repository,omitempty"`
	Auth       Auth   `yaml:"auth,omitempty"`
}

func (t Target) String() string {
	var target string
	if t.Repository != "" {
		target = "/" + t.Repository + target
	}

	if t.Host != "" {
		target = "/" + t.Host + target
	}

	target = strings.TrimLeft(target, "/")

	return target
}

// SourceImage is a source container image
type SourceImage struct {
	Repository string `yaml:"repository"`
	Host       string `yaml:"host,omitempty"`
	Target     Target `yaml:"target,omitempty"`
	Tag        string `yaml:"tag,omitempty"`
	Digest     string `yaml:"digest,omitempty"`
	Auth       Auth   `yaml:"auth,omitempty"`
}

// String returns the source image including its tag
func (c SourceImage) String() string {
	var source string
	if c.Tag != "" {
		source = ":" + c.Tag
	} else if c.Digest != "" {
		source = "@" + c.Digest
	}

	if c.Repository != "" {
		source = "/" + c.Repository + source
	}

	if c.Host != "" {
		source = "/" + c.Host + source
	}

	source = strings.TrimLeft(source, "/")

	return source
}

// TargetImage returns the target image includes its tag
func (c SourceImage) TargetImage() string {
	var target string
	if c.Tag != "" {
		target = ":" + c.Tag
	} else if c.Digest != "" {
		target = strings.ReplaceAll(c.Digest, "sha256:", "")
		target = ":" + target
	}

	if c.Repository != "" {
		target = "/" + c.Repository + target
	}

	if c.Target.Repository != "" {
		target = "/" + c.Target.Repository + target
	}

	if c.Target.Host != "" {
		target = "/" + c.Target.Host + target
	}

	target = strings.TrimLeft(target, "/")

	return target
}

// Auth is a username and password to log into a registry
type Auth struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// NewManifest returns a new image manifest
func NewManifest(target string) Manifest {
	targetPath := docker.RegistryPath(target)

	manifestTarget := Target{
		Host:       targetPath.Host(),
		Repository: targetPath.Repository(),
	}

	manifest := Manifest{
		Target: manifestTarget,
	}

	return manifest
}

// NewAutodetectManifest returns a new image manifest with images found in the repository
func NewAutodetectManifest(target string, path string) (Manifest, error) {
	manifest := NewManifest(target)

	foundImages, err := getImagesFromKubernetesManifests(path, manifest.Target)
	if err != nil {
		return Manifest{}, fmt.Errorf("get from kubernetes manifests: %w", err)
	}

	manifest.Images = foundImages

	return manifest, nil
}

// GetManifest returns the current manifest file in the working directory
func GetManifest(path string) (Manifest, error) {
	manifestLocation := getManifestLocation(path)
	manifestContents, err := ioutil.ReadFile(manifestLocation)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading manifest: %w", err)
	}

	manifest, err := marshalManifest(manifestContents)
	if err != nil {
		return Manifest{}, fmt.Errorf("marshal manifest: %w", err)
	}

	return manifest, nil
}

func marshalManifest(manifestContents []byte) (Manifest, error) {
	var manifest Manifest
	if err := yaml.Unmarshal(manifestContents, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("unmarshal current manifest: %w", err)
	}

	for i := range manifest.Images {
		if manifest.Images[i].Target.Host == "" {
			manifest.Images[i].Target = manifest.Target
		}
	}

	return manifest, nil
}

func writeManifest(manifest Manifest, path string) error {
	imageManifestContents, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshal image manifest: %w", err)
	}
	imageManifestContents = bytes.ReplaceAll(imageManifestContents, []byte(`"`), []byte(""))

	manifestLocation := getManifestLocation(path)
	if err := ioutil.WriteFile(manifestLocation, imageManifestContents, os.ModePerm); err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	return nil
}

func getManifestLocation(path string) string {
	const defaultManifestFileName = ".images.yaml"

	var manifestLocation string
	if strings.Contains(path, ".yaml") || strings.Contains(path, ".yml") {
		manifestLocation = path
	} else {
		manifestLocation = filepath.Join(path, defaultManifestFileName)
	}

	return manifestLocation
}
