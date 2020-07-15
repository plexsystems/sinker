package manifest

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	kubeyaml "github.com/ghodss/yaml"
	"github.com/plexsystems/sinker/internal/docker"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gopkg.in/yaml.v2"
)

// Manifest is a collection of images to sync
type Manifest struct {
	Target  Target   `yaml:"target"`
	Sources []Source `yaml:"sources,omitempty"`
}

// New returns a new image manifest
func New(target string) Manifest {
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

// NewWithAutodetect returns a new image manifest with images found in the repository
func NewWithAutodetect(target string, path string) (Manifest, error) {
	manifest := New(target)

	foundImages, err := getImagesFromKubernetesManifests(path, manifest.Target)
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

func getImagesFromKubernetesManifests(path string, target Target) ([]Source, error) {
	files, err := getYamlFiles(path)
	if err != nil {
		return nil, fmt.Errorf("get yaml files: %w", err)
	}

	yamlFiles, err := splitYamlFiles(files)
	if err != nil {
		return nil, fmt.Errorf("split yaml files: %w", err)
	}

	var imageList []string
	for _, yamlFile := range yamlFiles {
		images, err := getImagesFromYamlFile(yamlFile)
		if err != nil {
			return nil, fmt.Errorf("get images from yaml: %w", err)
		}

		imageList = append(imageList, images...)
	}

	var dedupedImageList []string
	for _, image := range imageList {
		if !contains(dedupedImageList, image) {
			dedupedImageList = append(dedupedImageList, image)
		}
	}

	marshalledImages, err := marshalImages(dedupedImageList, target)
	if err != nil {
		return nil, fmt.Errorf("marshal images: %w", err)
	}

	return marshalledImages, nil
}

func getYamlFiles(path string) ([]string, error) {
	var files []string
	err := filepath.Walk(path, func(currentFilePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk path: %w", err)
		}

		if fileInfo.IsDir() && fileInfo.Name() == ".git" {
			return filepath.SkipDir
		}

		if fileInfo.IsDir() {
			return nil
		}

		if filepath.Ext(currentFilePath) != ".yaml" && filepath.Ext(currentFilePath) != ".yml" {
			return nil
		}

		files = append(files, currentFilePath)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func splitYamlFiles(files []string) ([][]byte, error) {
	var yamlFiles [][]byte
	for _, file := range files {
		fileContent, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}

		var lineBreak string
		if bytes.Contains(fileContent, []byte("\r\n")) && runtime.GOOS == "windows" {
			lineBreak = "\r\n"
		} else {
			lineBreak = "\n"
		}

		individualYamlFiles := bytes.Split(fileContent, []byte(lineBreak+"---"+lineBreak))

		yamlFiles = append(yamlFiles, individualYamlFiles...)
	}

	return yamlFiles, nil
}

func marshalImages(images []string, target Target) ([]Source, error) {
	var containerImages []Source
	for _, image := range images {
		path := docker.RegistryPath(image)

		sourceHost := getSourceHostFromRepository(path.Repository())

		sourceRepository := path.Repository()
		sourceRepository = strings.Replace(sourceRepository, target.Repository, "", 1)
		sourceRepository = strings.TrimLeft(sourceRepository, "/")

		source := Source{
			Host:       sourceHost,
			Repository: sourceRepository,
			Tag:        path.Tag(),
		}

		containerImages = append(containerImages, source)
	}

	return containerImages, nil
}

func getSourceHostFromRepository(repository string) string {
	repositoryMappings := map[string]string{
		"kubernetes-ingress-controller": "quay.io",
		"coreos":                        "quay.io",
		"open-policy-agent":             "quay.io",
		"twistlock":                     "registry.twistlock.com",
	}

	for repositorySegment, host := range repositoryMappings {
		if strings.Contains(repository, repositorySegment) {
			return host
		}
	}

	return ""
}

func getImagesFromYamlFile(yamlFile []byte) ([]string, error) {

	// If the yaml does not contain a TypeMeta, it will not be a valid
	// Kubernetes resource and can be assumed to have no images
	var typeMeta metav1.TypeMeta
	if err := kubeyaml.Unmarshal(yamlFile, &typeMeta); err != nil {
		return []string{}, nil
	}

	if typeMeta.Kind == "Prometheus" {
		prometheusImages, err := getPrometheusImages(yamlFile)
		if err != nil {
			return nil, fmt.Errorf("get prometheus images: %w", err)
		}

		return prometheusImages, nil
	}

	if typeMeta.Kind == "Alertmanager" {
		alertmanagerImages, err := getAlertmanagerImages(yamlFile)
		if err != nil {
			return nil, fmt.Errorf("get alertmanager images: %w", err)
		}

		return alertmanagerImages, nil
	}

	type BaseSpec struct {
		Template corev1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`
	}

	type BaseType struct {
		Spec BaseSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	}

	var contents BaseType
	if err := kubeyaml.Unmarshal(yamlFile, &contents); err != nil {
		return []string{}, nil
	}

	var images []string
	images = append(images, getImagesFromContainers(contents.Spec.Template.Spec.InitContainers)...)
	images = append(images, getImagesFromContainers(contents.Spec.Template.Spec.Containers)...)

	return images, nil
}

func getPrometheusImages(yamlFile []byte) ([]string, error) {
	var prometheus promv1.Prometheus
	if err := kubeyaml.Unmarshal(yamlFile, &prometheus); err != nil {
		return nil, fmt.Errorf("unmarshal prometheus: %w", err)
	}

	var prometheusImage string
	if prometheus.Spec.BaseImage != "" {
		prometheusImage = prometheus.Spec.BaseImage + ":" + prometheus.Spec.Version
	} else {
		prometheusImage = *prometheus.Spec.Image
	}

	var images []string
	images = append(images, getImagesFromContainers(prometheus.Spec.Containers)...)
	images = append(images, getImagesFromContainers(prometheus.Spec.InitContainers)...)
	images = append(images, prometheusImage)

	return images, nil
}

func getAlertmanagerImages(yamlFile []byte) ([]string, error) {
	var alertmanager promv1.Alertmanager
	if err := kubeyaml.Unmarshal(yamlFile, &alertmanager); err != nil {
		return nil, fmt.Errorf("unmarshal alertmanager: %w", err)
	}

	var alertmanagerImage string
	if alertmanager.Spec.BaseImage != "" {
		alertmanagerImage = alertmanager.Spec.BaseImage + ":" + alertmanager.Spec.Version
	} else {
		alertmanagerImage = *alertmanager.Spec.Image
	}

	var images []string
	images = append(images, getImagesFromContainers(alertmanager.Spec.Containers)...)
	images = append(images, getImagesFromContainers(alertmanager.Spec.InitContainers)...)
	images = append(images, alertmanagerImage)

	return images, nil
}

func getImagesFromContainers(containers []corev1.Container) []string {
	var images []string
	for _, container := range containers {
		images = append(images, container.Image)

		for _, arg := range container.Args {
			if !strings.Contains(arg, ":") || strings.Contains(arg, "=:") {
				continue
			}

			argTokens := strings.Split(arg, "=")
			images = append(images, argTokens[1])
		}
	}

	return images
}

func contains(images []string, image string) bool {
	for _, currentImage := range images {
		if strings.EqualFold(currentImage, image) {
			return true
		}
	}

	return false
}
