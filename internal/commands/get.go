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
	manyaml "github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const manifestFileName = ".images.yaml"

// ImageManifest is a collection of images to mirror
type ImageManifest struct {
	Mirror Mirror           `yaml:"mirror"`
	Images []ContainerImage `yaml:"images"`
}

// Mirror is the registry and repository to mirror images to
type Mirror struct {
	Registry   string `yaml:"registry"`
	Repository string `yaml:"repository,omitempty"`
}

func newMirror(mirrorLocation string) Mirror {
	var registry string
	var repository string
	mirrorTokens := strings.Split(mirrorLocation, "/")
	if len(mirrorTokens) > 1 {
		registry = mirrorTokens[0]
		repository = strings.Join(mirrorTokens[1:], "/")
	} else {
		registry = mirrorLocation
	}

	mirror := Mirror{
		Registry:   registry,
		Repository: repository,
	}

	return mirror
}

func (m Mirror) String() string {
	return m.Registry + "/" + m.Repository
}

// ContainerImage is a container image found in a repository
type ContainerImage struct {
	Repository     string `yaml:"repository"`
	Version        string `yaml:"version"`
	OriginRegistry string `yaml:"origin,omitempty"`
	Auth           Auth   `yaml:"auth,omitempty"`
}

// Auth is a username and password to log into the registry
type Auth struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

func (c ContainerImage) String() string {
	return c.Repository + ":" + c.Version
}

// Origin returns the origin image
func (c ContainerImage) Origin() string {
	if c.OriginRegistry == "" {
		return c.Repository + ":" + c.Version
	}

	return c.OriginRegistry + "/" + c.Repository + ":" + c.Version
}

func newGetCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "get",
		Short: "Gets the images found in Kubernetes manifests",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("mirror", cmd.Flags().Lookup("mirror")); err != nil {
				return fmt.Errorf("bind flag: %w", err)
			}

			if err := runGetCommand("."); err != nil {
				return fmt.Errorf("list: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("mirror", "m", "", "The repository to mirror images to (e.g. organization.com/repo)")

	return &cmd
}

func runGetCommand(path string) error {
	var imageManifest ImageManifest
	if _, err := os.Stat(manifestFileName); !os.IsNotExist(err) {
		imageManifest, err = getManifest(path)
		if err != nil {
			return fmt.Errorf("get manifest: %w", err)
		}
	}

	if viper.GetString("mirror") != "" {
		imageManifest.Mirror = newMirror(viper.GetString("mirror"))
	}

	foundImages, err := getFromKubernetesManifests(path, imageManifest.Mirror)
	if err != nil {
		return fmt.Errorf("get from kubernetes manifests: %w", err)
	}

	if _, err := os.Stat(manifestFileName); os.IsNotExist(err) {
		imageManifest.Images = foundImages
	} else {
		imageManifest.Images, err = getUpdatedImages(foundImages, imageManifest)
		if err != nil {
			return fmt.Errorf("get updated images: %w", err)
		}
	}

	if err := writeManifest(imageManifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func getManifest(path string) (ImageManifest, error) {
	imageManifestContents, err := ioutil.ReadFile(manifestFileName)
	if err != nil {
		return ImageManifest{}, fmt.Errorf("reading manifest: %w", err)
	}

	var currentImageManifest ImageManifest
	if err := yaml.Unmarshal(imageManifestContents, &currentImageManifest); err != nil {
		return ImageManifest{}, fmt.Errorf("unmarshal current manifest: %w", err)
	}

	return currentImageManifest, nil
}

func getFromKubernetesManifests(path string, mirror Mirror) ([]ContainerImage, error) {
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
		if err := manyaml.Unmarshal(yamlFile, &typeMeta); err != nil {
			continue
		}

		if typeMeta.Kind == "Prometheus" {
			var prometheus promv1.Prometheus
			if err := manyaml.Unmarshal(yamlFile, &prometheus); err != nil {
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
			if err := manyaml.Unmarshal(yamlFile, &alertmanager); err != nil {
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
		if err := manyaml.Unmarshal(yamlFile, &contents); err != nil {
			continue
		}

		imageList = append(imageList, getImagesFromContainers(contents.Spec.Template.Spec.InitContainers)...)
		imageList = append(imageList, getImagesFromContainers(contents.Spec.Template.Spec.Containers)...)
	}

	dedupedImageList := dedupeImages(imageList)
	marshalledImages, err := marshalImages(dedupedImageList, mirror)
	if err != nil {
		return nil, fmt.Errorf("marshal images: %w", err)
	}

	return marshalledImages, nil
}

func marshalImages(images []string, mirror Mirror) ([]ContainerImage, error) {
	var containerImages []ContainerImage
	for _, image := range images {
		imageReference, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return nil, fmt.Errorf("parse image: %w", err)
		}
		imageReference = reference.TagNameOnly(imageReference)

		imageRepository := reference.Path(imageReference)
		imageRepository = strings.Replace(imageRepository, mirror.Repository+"/", "", 1)

		imageVersion := strings.Split(imageReference.String(), ":")[1]

		originRegistry := getOriginRegistryFromRepository(imageRepository)

		containerImage := ContainerImage{
			Repository:     imageRepository,
			Version:        imageVersion,
			OriginRegistry: originRegistry,
		}

		containerImages = append(containerImages, containerImage)
	}

	return containerImages, nil
}

func getOriginRegistryFromRepository(repository string) string {
	repositoryMappings := map[string]string{
		"kubernetes-ingress-controller": "quay.io",
		"coreos":                        "quay.io",
		"twistlock":                     "registry.twistlock.com",
	}

	originRegistry := "docker.io"
	for repositorySegment, registry := range repositoryMappings {
		if strings.Contains(repository, repositorySegment) {
			originRegistry = registry
		}
	}

	return originRegistry
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

func getUpdatedImages(images []ContainerImage, manifest ImageManifest) ([]ContainerImage, error) {
	var updatedImages []ContainerImage
	for _, updatedImage := range images {
		for _, currentImage := range manifest.Images {
			if currentImage.Repository == updatedImage.Repository {
				updatedImage.Auth = currentImage.Auth
				updatedImage.Repository = currentImage.Repository
				updatedImage.OriginRegistry = currentImage.OriginRegistry
			}
		}

		updatedImages = append(updatedImages, updatedImage)
	}

	return updatedImages, nil
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
