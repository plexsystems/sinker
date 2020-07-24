package manifest

import (
	"bufio"
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

// Manifest contains all of the sources to push to a target registry.
type Manifest struct {
	Target  Target   `yaml:"target"`
	Sources []Source `yaml:"sources,omitempty"`
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

func (m Manifest) Update(images []string) Manifest {
	var updatedSources []Source
	for _, updatedImage := range images {
		updatedRegistryPath := docker.RegistryPath(updatedImage)

		updatedSource := Source{
			Tag:    updatedRegistryPath.Tag(),
			Digest: updatedRegistryPath.Digest(),
		}

		// Attempt to find the source in the current manifest. If found, it's possible
		// to re-use already set values, such as the host of the source registry.
		//
		// In the event the source cannot be found in the manifest, we must rely on
		// trying to find the source registry from the repository the image is sourced from.
		//
		// This is more of a nice-to-have. The worst case is that we get it wrong and the
		// user has to update the host value to the correct one. Once defined in the manifest
		// we can continue to use the host that was set.
		foundSource, exists := m.findSourceInManifest(updatedImage)
		if !exists {
			updatedSource.Host = getSourceHostFromRepository(updatedRegistryPath.Repository())

			updatedRepository := updatedRegistryPath.Repository()
			updatedRepository = strings.Replace(updatedRepository, m.Target.Repository, "", 1)
			updatedRepository = strings.TrimLeft(updatedRepository, "/")
			updatedSource.Repository = updatedRepository

			updatedSources = append(updatedSources, updatedSource)
			continue
		}

		updatedSource.Repository = foundSource.Repository
		updatedSource.Host = foundSource.Host
		updatedSource.Auth = foundSource.Auth

		// If the target host (or repository) of the source does not match the manifest
		// target host (or repository), it has been modified by the user.
		//
		// To preserve the current settings, set the manifest host and repository values
		// to the ones present in the current manifest.
		if foundSource.Target.Host != m.Target.Host {
			updatedSource.Target.Host = foundSource.Target.Host
		}
		if foundSource.Target.Repository != m.Target.Repository {
			updatedSource.Target.Repository = foundSource.Target.Repository
		}

		updatedSources = append(updatedSources, updatedSource)
	}

	updatedManifest := Manifest{
		Target:  m.Target,
		Sources: updatedSources,
	}

	return updatedManifest
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
		if hostSupportsNestedRepositories(s.Target.Host) {
			target = "/" + s.Repository + target
		} else {
			target = "/" + filepath.Base(s.Repository) + target
		}
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
	images = dedupeImages(images)

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

// GetImagesFromStandardInput gets a list of images passed in by standard input.
func GetImagesFromStandardInput() ([]string, error) {
	standardInReader := ioutil.NopCloser(bufio.NewReader(os.Stdin))
	contents, err := ioutil.ReadAll(standardInReader)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	images := strings.Split(string(contents), " ")
	images = dedupeImages(images)
	return images, nil
}

func getSourceHostFromRepository(repository string) string {
	repositoryMappings := map[string]string{
		"kubernetes-ingress-controller": "quay.io",
		"coreos":                        "quay.io",
		"open-policy-agent":             "quay.io",

		"twistlock": "registry.twistlock.com",

		"etcd":                    "k8s.gcr.io",
		"kube-apiserver":          "k8s.gcr.io",
		"coredns":                 "k8s.gcr.io",
		"kube-proxy":              "k8s.gcr.io",
		"kube-scheduler":          "k8s.gcr.io",
		"kube-controller-manager": "k8s.gcr.io",
	}

	for repositorySegment, host := range repositoryMappings {
		if strings.Contains(repository, repositorySegment) {
			return host
		}
	}

	// An empty host refers to an image that is on Docker Hub.
	return ""
}

func (m Manifest) findSourceInManifest(image string) (Source, bool) {
	for _, currentSource := range m.Sources {
		imagePath := docker.RegistryPath(image)
		sourceImagePath := docker.RegistryPath(currentSource.Image())
		targetImagePath := docker.RegistryPath(currentSource.TargetImage())

		if imagePath.Host() == sourceImagePath.Host() && imagePath.Repository() == sourceImagePath.Repository() {
			return currentSource, true
		}

		if imagePath.Host() == targetImagePath.Host() && imagePath.Repository() == targetImagePath.Repository() {
			return currentSource, true
		}
	}

	return Source{}, false
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

func hostSupportsNestedRepositories(host string) bool {

	// Google Container Registry (GCR)
	if strings.Contains(host, "gcr.io") {
		return false
	}

	// Quay.io
	if strings.Contains(host, "quay.io") {
		return false
	}

	// Docker Registry (Docker Hub)
	// An empty host is assumed to be Docker Hub.
	if strings.Contains(host, "docker.io") || host == "" {
		return false
	}

	return true
}

func dedupeImages(images []string) []string {
	var dedupedImages []string
	for _, image := range images {
		if !contains(dedupedImages, image) {
			dedupedImages = append(dedupedImages, image)
		}
	}

	return dedupedImages
}
