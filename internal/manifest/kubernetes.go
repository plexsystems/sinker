package manifest

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/plexsystems/sinker/internal/docker"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	kubeyaml "github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetImagesFromKubernetesManifests returns all images found in Kubernetes manifests
// that are located at the specified path.
func GetImagesFromKubernetesManifests(path string, target Target) ([]Source, error) {
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
		fileContents, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}

		var lineBreak string
		if bytes.Contains(fileContents, []byte("\r\n")) && runtime.GOOS == "windows" {
			lineBreak = "\r\n"
		} else {
			lineBreak = "\n"
		}

		individualYamlFiles := bytes.Split(fileContents, []byte(lineBreak+"---"+lineBreak))

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
			Digest:     path.Digest(),
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

	// An empty host refers to an image that is on Docker Hub.
	return ""
}

func getImagesFromYamlFile(yamlFile []byte) ([]string, error) {

	// If the yaml does not contain a TypeMeta, it will not be a valid
	// Kubernetes resource and can be assumed to have no images.
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
