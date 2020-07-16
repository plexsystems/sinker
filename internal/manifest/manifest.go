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

// Manifest is a collection of images to sync
type Manifest struct {
	Target  Target   `yaml:"target"`
	Sources []Source `yaml:"sources,omitempty"`
}

// New returns a new image manifest
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

// NewWithAutodetect returns a new image manifest with images found in the repository
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

// Get returns the manifest at the specified path
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

// Write writes the contents of the manifest file to the specified path
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

// Auth is a username and password to log into a registry
type Auth struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
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

// EncodedAuth returns the Base64 encoded auth for the target registry
func (t Target) EncodedAuth() (string, error) {
	if t.Auth.Password != "" {
		auth, err := docker.GetEncodedBasicAuth(t.Auth.Username, t.Auth.Password)
		if err != nil {
			return "", fmt.Errorf("get encoded auth: %w", err)
		}

		return auth, nil
	}

	authHost := getAuthHostFromRegistryHost(t.Host)
	auth, err := docker.GetEncodedAuthForHost(authHost)
	if err != nil {
		return "", fmt.Errorf("get encoded auth for host: %w", err)
	}

	return auth, nil
}

// Source is a container image in the manifest
type Source struct {
	Repository string `yaml:"repository"`
	Host       string `yaml:"host,omitempty"`
	Target     Target `yaml:"target,omitempty"`
	Tag        string `yaml:"tag,omitempty"`
	Digest     string `yaml:"digest,omitempty"`
	Auth       Auth   `yaml:"auth,omitempty"`
}

// Image returns the source image including its tag
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

// TargetImage returns the target image includes its tag
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

// EncodedAuth returns the Base64 encoded auth for the source registry
func (s Source) EncodedAuth() (string, error) {
	if s.Auth.Password != "" {
		auth, err := docker.GetEncodedBasicAuth(s.Auth.Username, s.Auth.Password)
		if err != nil {
			return "", fmt.Errorf("get encoded auth: %w", err)
		}

		return auth, nil
	}

	authHost := getAuthHostFromRegistryHost(s.Host)
	auth, err := docker.GetEncodedAuthForHost(authHost)
	if err != nil {
		return "", fmt.Errorf("get encoded auth for host: %w", err)
	}

	return auth, nil
}

func getAuthHostFromRegistryHost(host string) string {
	if host == "" || host == "docker.io" {
		return "https://index.docker.io/v1/"
	}

	return host
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
