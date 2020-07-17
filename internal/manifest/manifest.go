package manifest

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

// A Manifest contains all of the sources to push to a target registry.
type Manifest struct {
	Target  Target   `yaml:"target"`
	Sources []Source `yaml:"sources,omitempty"`
}

// New returns an empty Manifest with the target set to the
// specified host and repository.
func New(host string, repository string) Manifest {
	manifestTarget := Target{
		Host:       host,
		Repository: repository,
	}

	manifest := Manifest{
		Target: manifestTarget,
	}

	return manifest
}

// NewWithAutodetect returns a Manifest populated with the images found in the specified path.
// The target of the Manifest will be set to the specified host and repository.
func NewWithAutodetect(host string, repository string, path string) (Manifest, error) {
	manifest := New(host, repository)

	target := Target{
		Host:       host,
		Repository: repository,
	}

	foundImages, err := GetImagesFromKubernetesManifests(path, target)
	if err != nil {
		return Manifest{}, fmt.Errorf("get from kubernetes manifests: %w", err)
	}

	manifest.Sources = foundImages

	return manifest, nil
}

// Get returns the Manifest found at the specified path.
// An error is returned when a Manifest cannot be found.
func Get(path string) (Manifest, error) {
	manifestLocation := getManifestLocation(path)
	manifestContents, err := ioutil.ReadFile(manifestLocation)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(manifestContents, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("unmarshal current manifest: %w", err)
	}

	for i := range manifest.Sources {
		if manifest.Sources[i].Target.Host == "" {
			manifest.Sources[i].Target = manifest.Target
		}
	}

	return manifest, nil
}

// Write writes the contents of the manifest file to the specified path.
func (m Manifest) Write(path string) error {
	imageManifestContents, err := yaml.Marshal(&m)
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

// Auth is a username and password to authenticate to a registry.
type Auth struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// Target is the target registry where the images defined in
// the manifest will be pushed to.
type Target struct {
	Host       string `yaml:"host,omitempty"`
	Repository string `yaml:"repository,omitempty"`
	Auth       Auth   `yaml:"auth,omitempty"`
}

// EncodedAuth returns the Base64 encoded auth for the target registry.
func (t Target) EncodedAuth() (string, error) {
	if t.Auth.Password != "" {
		auth, err := docker.GetEncodedBasicAuth(t.Auth.Username, t.Auth.Password)
		if err != nil {
			return "", fmt.Errorf("get encoded auth: %w", err)
		}

		return auth, nil
	}

	auth, err := docker.GetEncodedAuthForHost(t.Host)
	if err != nil {
		return "", fmt.Errorf("get encoded auth for host: %w", err)
	}

	return auth, nil
}

// Source is a container image in the manifest.
type Source struct {
	Repository string `yaml:"repository"`
	Host       string `yaml:"host,omitempty"`
	Target     Target `yaml:"target,omitempty"`
	Tag        string `yaml:"tag,omitempty"`
	Digest     string `yaml:"digest,omitempty"`
	Auth       Auth   `yaml:"auth,omitempty"`
}

// Image returns the source image including its tag or digest.
func (s Source) Image() string {
	var source string
	if s.Tag != "" {
		source = ":" + s.Tag
	} else if s.Digest != "" {
		source = "@" + s.Digest
	}

	if s.Repository != "" {
		source = "/" + s.Repository + source
	}

	if s.Host != "" {
		source = "/" + s.Host + source
	}

	source = strings.TrimLeft(source, "/")

	return source
}

// TargetImage returns the target image including its tag or digest.
func (s Source) TargetImage() string {
	var target string
	if s.Tag != "" {
		target = ":" + s.Tag
	} else if s.Digest != "" {
		target = strings.ReplaceAll(s.Digest, "sha256:", "")
		target = ":" + target
	}

	if s.Repository != "" {
		target = "/" + s.Repository + target
	}

	if s.Target.Repository != "" {
		target = "/" + s.Target.Repository + target
	}

	if s.Target.Host != "" {
		target = "/" + s.Target.Host + target
	}

	target = strings.TrimLeft(target, "/")

	return target
}

// EncodedAuth returns the Base64 encoded auth for the source registry.
func (s Source) EncodedAuth() (string, error) {
	if s.Auth.Password != "" {
		auth, err := docker.GetEncodedBasicAuth(s.Auth.Username, s.Auth.Password)
		if err != nil {
			return "", fmt.Errorf("get encoded auth: %w", err)
		}

		return auth, nil
	}

	auth, err := docker.GetEncodedAuthForHost(s.Host)
	if err != nil {
		return "", fmt.Errorf("get encoded auth for host: %w", err)
	}

	return auth, nil
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
