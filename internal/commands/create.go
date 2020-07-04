package commands

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/docker/distribution/reference"
	kubeyaml "github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const manifestFileName = ".images.yaml"

// ImageManifest is a collection of images to sync
type ImageManifest struct {
	Target Target           `yaml:"target"`
	Images []ContainerImage `yaml:"images,omitempty"`
}

// Target is the registry and repository to sync images to
type Target struct {
	Registry   string `yaml:"registry"`
	Repository string `yaml:"repository,omitempty"`
}

func newTarget(target string) Target {
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

	return targetLocation
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

// Auth is a username and password to log into a registry
type Auth struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// RepositoryWithTag returns the full repository path including the tag
func (c ContainerImage) RepositoryWithTag() string {
	return c.Repository + ":" + c.Version
}

// Source returns the source image
func (c ContainerImage) Source() string {
	if c.SourceRegistry == "docker.io" && !strings.Contains(c.Repository, "/") {
		return c.SourceRegistry + "/library/" + c.RepositoryWithTag()
	}

	if c.SourceRegistry == "" {
		return c.RepositoryWithTag()
	}

	return c.SourceRegistry + "/" + c.RepositoryWithTag()
}

// Target returns the target image
func (c ContainerImage) Target(target Target) string {
	return target.String() + "/" + c.RepositoryWithTag()
}

func newCreateCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "create <source>",
		Short: "Create a new image manifest in the current directory",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("target", cmd.Flags().Lookup("target")); err != nil {
				return fmt.Errorf("bind target flag: %w", err)
			}

			var path string
			if len(args) > 0 {
				path = args[0]
			}

			if err := runCreateCommand(path); err != nil {
				return fmt.Errorf("create: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("target", "t", "", "The target repository to sync images to (e.g. organization.com/repo)")

	return &cmd
}

func runCreateCommand(path string) error {
	if _, err := os.Stat(manifestFileName); !os.IsNotExist(err) {
		return fmt.Errorf("manifest %s already exists in current directory", manifestFileName)
	}

	if viper.GetString("target") == "" {
		return fmt.Errorf("%s flag must be set when creating initial manifest", "--target")
	}

	var imageManifest ImageManifest
	imageManifest.Target = newTarget(viper.GetString("target"))

	if path != "" {
		foundImages, err := getFromKubernetesManifests(path, imageManifest.Target)
		if err != nil {
			return fmt.Errorf("get from kubernetes manifests: %w", err)
		}

		imageManifest.Images = foundImages
	}

	if err := writeManifest(imageManifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func writeManifest(imageManifest ImageManifest) error {
	imageManifestContents, err := yaml.Marshal(&imageManifest)
	if err != nil {
		return fmt.Errorf("marshal image manifest: %w", err)
	}
	imageManifestContents = bytes.ReplaceAll(imageManifestContents, []byte(`"`), []byte(""))

	if err := ioutil.WriteFile(manifestFileName, imageManifestContents, os.ModePerm); err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	return nil
}

func autoDetectSourceRegistry(repository string) string {
	repositoryMappings := map[string]string{
		"kubernetes-ingress-controller": "quay.io",
		"coreos":                        "quay.io",
		"open-policy-agent":             "quay.io",
		"twistlock":                     "registry.twistlock.com",
	}

	sourceRegistry := "docker.io"
	for repositorySegment, registry := range repositoryMappings {
		if strings.Contains(repository, repositorySegment) {
			sourceRegistry = registry
		}
	}

	return sourceRegistry
}

func marshalImages(images []string, target Target) ([]ContainerImage, error) {
	var containerImages []ContainerImage
	for _, image := range images {
		imageReference, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return nil, fmt.Errorf("parse image: %w", err)
		}
		imageReference = reference.TagNameOnly(imageReference)

		imageRepository := reference.Path(imageReference)
		if target.Repository != "" {
			imageRepository = strings.Replace(imageRepository, target.Repository+"/", "", 1)
			imageRepository = strings.Replace(imageRepository, "library/", "", 1)
		} else {
			imageRepository = strings.Replace(imageRepository, target.Repository, "", 1)
		}

		imageVersion := strings.Split(imageReference.String(), ":")[1]

		sourceRegistry := autoDetectSourceRegistry(imageRepository)

		containerImage := ContainerImage{
			Repository:     imageRepository,
			Version:        imageVersion,
			SourceRegistry: sourceRegistry,
		}

		containerImages = append(containerImages, containerImage)
	}

	return containerImages, nil
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

		individualYamlFiles := doSplit(fileContent)

		yamlFiles = append(yamlFiles, individualYamlFiles...)
	}

	return yamlFiles, nil
}

func contains(images []string, image string) bool {
	for _, currentImage := range images {
		if strings.EqualFold(currentImage, image) {
			return true
		}
	}

	return false
}

func dedupeImages(images []string) []string {
	var dedupedImageList []string
	for _, image := range images {
		if !contains(dedupedImageList, image) {
			dedupedImageList = append(dedupedImageList, image)
		}
	}

	return dedupedImageList
}

func doSplit(data []byte) [][]byte {
	linebreak := "\n"
	windowsLineEnding := bytes.Contains(data, []byte("\r\n"))
	if windowsLineEnding && runtime.GOOS == "windows" {
		linebreak = "\r\n"
	}

	return bytes.Split(data, []byte(linebreak+"---"+linebreak))
}

func getFromKubernetesManifests(path string, target Target) ([]ContainerImage, error) {
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
		var typeMeta metav1.TypeMeta
		if err := kubeyaml.Unmarshal(yamlFile, &typeMeta); err != nil {
			continue
		}

		if typeMeta.Kind == "Prometheus" {
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

			if len(prometheus.Spec.Containers) > 0 {
				imageList = append(imageList, getImagesFromContainers(prometheus.Spec.Containers)...)
			}

			if len(prometheus.Spec.InitContainers) > 0 {
				imageList = append(imageList, getImagesFromContainers(prometheus.Spec.InitContainers)...)
			}

			imageList = append(imageList, prometheusImage)
			continue
		}

		if typeMeta.Kind == "Alertmanager" {
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

			if len(alertmanager.Spec.Containers) > 0 {
				imageList = append(imageList, getImagesFromContainers(alertmanager.Spec.Containers)...)
			}

			if len(alertmanager.Spec.InitContainers) > 0 {
				imageList = append(imageList, getImagesFromContainers(alertmanager.Spec.InitContainers)...)
			}

			imageList = append(imageList, alertmanagerImage)
			continue
		}

		type BaseSpec struct {
			Template corev1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`
		}

		type BaseType struct {
			Spec BaseSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
		}

		var contents BaseType
		if err := kubeyaml.Unmarshal(yamlFile, &contents); err != nil {
			continue
		}

		imageList = append(imageList, getImagesFromContainers(contents.Spec.Template.Spec.InitContainers)...)
		imageList = append(imageList, getImagesFromContainers(contents.Spec.Template.Spec.Containers)...)
	}

	dedupedImageList := dedupeImages(imageList)
	marshalledImages, err := marshalImages(dedupedImageList, target)
	if err != nil {
		return nil, fmt.Errorf("marshal images: %w", err)
	}

	return marshalledImages, nil
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
