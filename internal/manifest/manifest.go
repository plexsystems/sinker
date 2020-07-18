package manifest

import (
	"encoding/base64"
	"encoding/json"
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
	target := Target{
		Host:       host,
		Repository: repository,
	}

	manifest := Manifest{
		Target: target,
	}

	return manifest
}

// NewWithAutodetect returns a manifest populated with the images found at the specified path.
// The target of the manifest will be set to the specified host and repository.
func NewWithAutodetect(host string, repository string, path string) (Manifest, error) {
	manifest := New(host, repository)

	target := Target{
		Host:       host,
		Repository: repository,
	}

	images, err := GetImagesFromKubernetesManifests(path, target)
	if err != nil {
		return Manifest{}, fmt.Errorf("get from kubernetes manifests: %w", err)
	}

	manifest.Sources = images

	return manifest, nil
}

// Get returns the manifest found at the specified path.
func Get(path string) (Manifest, error) {
	manifestLocation := getManifestLocation(path)
	manifestContents, err := ioutil.ReadFile(manifestLocation)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(manifestContents, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("unmarshal manifest: %w", err)
	}

	// When a source in the manifest does not define its own target the default target
	// should be the target defined in the manifest.
	for s := range manifest.Sources {
		if manifest.Sources[s].Target.Host == "" {
			manifest.Sources[s].Target = manifest.Target
		}
	}

	return manifest, nil
}

// Write writes the contents of the manifest to disk at the specified path.
func (m Manifest) Write(path string) error {
	imageManifestContents, err := yaml.Marshal(&m)
	if err != nil {
		return fmt.Errorf("marshal image manifest: %w", err)
	}

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
		auth, err := getEncodedBasicAuth(os.Getenv(t.Auth.Username), os.Getenv(t.Auth.Password))
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
		auth, err := getEncodedBasicAuth(os.Getenv(s.Auth.Username), os.Getenv(s.Auth.Password))
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

// GetSourcesFromImages returns the given images as sources with the specified target.
func GetSourcesFromImages(images []string, target string) []Source {
	targetRegistryPath := docker.RegistryPath(target)
	sourceTarget := Target{
		Host:       targetRegistryPath.Host(),
		Repository: targetRegistryPath.Repository(),
	}

	var sources []Source
	for _, image := range images {
		registryPath := docker.RegistryPath(image)

		source := Source{
			Host:       registryPath.Host(),
			Target:     sourceTarget,
			Repository: registryPath.Repository(),
			Tag:        registryPath.Tag(),
			Digest:     registryPath.Digest(),
		}

		sources = append(sources, source)
	}

	return sources
}

func getManifestLocation(path string) string {
	const defaultManifestFileName = ".images.yaml"

	location := path
	if !strings.Contains(location, ".yaml") && !strings.Contains(location, ".yml") {
		location = filepath.Join(path, defaultManifestFileName)
	}

	return location
}

func getEncodedBasicAuth(username string, password string) (string, error) {
	authConfig := Auth{
		Username: username,
		Password: password,
	}
	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}
