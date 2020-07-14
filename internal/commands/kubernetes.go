package commands

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

func marshalImages(images []string, target Target) ([]SourceImage, error) {
	var containerImages []SourceImage
	for _, image := range images {
		path := docker.RegistryPath(image)

		sourceImage := SourceImage{
			Host:       path.Host(),
			Repository: path.Repository(),
			Tag:        path.Tag(),
		}

		containerImages = append(containerImages, sourceImage)
	}

	return containerImages, nil
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

func doSplit(data []byte) [][]byte {
	linebreak := "\n"
	windowsLineEnding := bytes.Contains(data, []byte("\r\n"))
	if windowsLineEnding && runtime.GOOS == "windows" {
		linebreak = "\r\n"
	}

	return bytes.Split(data, []byte(linebreak+"---"+linebreak))
}

func getFromKubernetesManifests(path string, target Target) ([]SourceImage, error) {
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
			prometheusImages, err := getPrometheusImages(yamlFile)
			if err != nil {
				return nil, fmt.Errorf("get alertmanager images: %w", err)
			}

			imageList = append(imageList, prometheusImages...)
			continue
		}

		if typeMeta.Kind == "Alertmanager" {
			alertmanagerImages, err := getAlertmanagerImages(yamlFile)
			if err != nil {
				return nil, fmt.Errorf("get alertmanager images: %w", err)
			}

			imageList = append(imageList, alertmanagerImages...)
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

func getPrometheusImages(yamlFile []byte) ([]string, error) {
	var images []string
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
		images = append(images, getImagesFromContainers(prometheus.Spec.Containers)...)
	}

	if len(prometheus.Spec.InitContainers) > 0 {
		images = append(images, getImagesFromContainers(prometheus.Spec.InitContainers)...)
	}

	images = append(images, prometheusImage)

	return images, nil
}

func getAlertmanagerImages(yamlFile []byte) ([]string, error) {
	var images []string
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
		images = append(images, getImagesFromContainers(alertmanager.Spec.Containers)...)
	}

	if len(alertmanager.Spec.InitContainers) > 0 {
		images = append(images, getImagesFromContainers(alertmanager.Spec.InitContainers)...)
	}

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
