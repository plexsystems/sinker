package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/plexsystems/sinker/internal/docker"

	kubeyaml "github.com/ghodss/yaml"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetImagesFromKubernetesManifests returns all images found in Kubernetes manifests
// that are located at the specified path.
func GetImagesFromKubernetesManifests(path string) ([]string, error) {
	resources, err := getResourceContentsFromYamlFiles(path)
	if err != nil {
		return nil, fmt.Errorf("get yaml files: %w", err)
	}

	images, err := GetImagesFromKubernetesResources(resources)
	if err != nil {
		return nil, fmt.Errorf("get images from resources: %w", err)
	}

	return images, nil
}

// GetImagesFromKubernetesResources returns all images found in Kubernetes resources that have
// already been read from the disk, or are being read from standard input.
func GetImagesFromKubernetesResources(resources []string) ([]string, error) {
	var splitResources []string
	for _, resourceContents := range resources {
		var lineBreak string
		if strings.Contains(resourceContents, "\r\n") && runtime.GOOS == "windows" {
			lineBreak = "\r\n"
		} else {
			lineBreak = "\n"
		}

		individualResources := strings.Split(resourceContents, lineBreak+"---"+lineBreak)
		splitResources = append(splitResources, individualResources...)
	}

	var imageList []string
	for _, resource := range splitResources {
		images, err := getImagesFromResource(resource)
		if err != nil {
			return nil, fmt.Errorf("get images from resource: %w", err)
		}

		imageList = append(imageList, images...)
	}

	imageList = dedupeImages(imageList)
	return imageList, nil
}

func getResourceContentsFromYamlFiles(path string) ([]string, error) {
	var filePaths []string
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

		filePaths = append(filePaths, currentFilePath)

		return nil
	})
	if err != nil {
		return nil, err
	}

	var fileContents []string
	for _, filePath := range filePaths {
		contents, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}

		fileContents = append(fileContents, string(contents))
	}

	return fileContents, nil
}

func getImagesFromResource(resource string) ([]string, error) {
	byteResource := []byte(resource)

	// If the resource does not contain a TypeMeta, it will not be a valid
	// Kubernetes resource and can be assumed to have no images.
	var typeMeta metav1.TypeMeta
	if err := kubeyaml.Unmarshal(byteResource, &typeMeta); err != nil {
		return []string{}, nil
	}

	if typeMeta.Kind == "Prometheus" {
		prometheusImages, err := getPrometheusImages(byteResource)
		if err != nil {
			return nil, fmt.Errorf("get prometheus images: %w", err)
		}

		return prometheusImages, nil
	}

	if typeMeta.Kind == "Alertmanager" {
		alertmanagerImages, err := getAlertmanagerImages(byteResource)
		if err != nil {
			return nil, fmt.Errorf("get alertmanager images: %w", err)
		}

		return alertmanagerImages, nil
	}

	if typeMeta.Kind == "Pod" {
		podImages, err := getPodImages(byteResource)
		if err != nil {
			return nil, fmt.Errorf("get pod images: %w", err)
		}

		return podImages, nil
	}

	if typeMeta.Kind == "CronJob" {
		cronJobImages, err := getCronJobImages(byteResource)
		if err != nil {
			return nil, fmt.Errorf("get cronjob images: %w", err)
		}

		return cronJobImages, nil
	}

	type BaseSpec struct {
		Template corev1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`
	}

	type BaseType struct {
		Spec BaseSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	}

	var contents BaseType
	if err := kubeyaml.Unmarshal(byteResource, &contents); err != nil {
		return []string{}, nil
	}

	var images []string
	images = append(images, getImagesFromContainers(contents.Spec.Template.Spec.InitContainers)...)
	images = append(images, getImagesFromContainers(contents.Spec.Template.Spec.Containers)...)

	return images, nil
}

func getPrometheusImages(resource []byte) ([]string, error) {
	var prometheus promv1.Prometheus
	if err := kubeyaml.Unmarshal(resource, &prometheus); err != nil {
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

func getAlertmanagerImages(resource []byte) ([]string, error) {
	var alertmanager promv1.Alertmanager
	if err := kubeyaml.Unmarshal(resource, &alertmanager); err != nil {
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

func getPodImages(resource []byte) ([]string, error) {
	var pod corev1.PodTemplateSpec
	if err := kubeyaml.Unmarshal(resource, &pod); err != nil {
		return nil, fmt.Errorf("unmarshal pod: %w", err)
	}

	var images []string
	images = append(images, getImagesFromContainers(pod.Spec.Containers)...)
	images = append(images, getImagesFromContainers(pod.Spec.InitContainers)...)

	return images, nil
}

func getCronJobImages(resource []byte) ([]string, error) {
	var cj batchv1beta1.CronJob
	if err := kubeyaml.Unmarshal(resource, &cj); err != nil {
		return nil, fmt.Errorf("unmarshal cronjob: %w", err)
	}

	var images []string
	images = append(images, getImagesFromContainers(cj.Spec.JobTemplate.Spec.Template.Spec.Containers)...)
	images = append(images, getImagesFromContainers(cj.Spec.JobTemplate.Spec.Template.Spec.InitContainers)...)

	return images, nil
}

func getImagesFromContainers(containers []corev1.Container) []string {
	var images []string

	imgStringsToFilter := []string{
		// Ignore token characters.
		"$",

		// Ignore image parameters that include a URL.
		"http://",
		"https://",

		// Ignore Envoy/Istio log parameters
		// (https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/run-envoy.html#debugging-envoy)
		":trace",
		":debug",
		":info",
		":warn",
		":error",
		":critical",
		":off",
	}

	// Looking for strings like 0.0.0.0:5332
	regexToFilter := []string{".*\\d*\\.\\d*:\\d"}

	for _, container := range containers {
		images = append(images, container.Image)

		for _, arg := range container.Args {
			var image string
			if strings.Contains(arg, "=") {
				image = strings.Split(arg, "=")[1]
			} else {
				image = arg
			}

			if !strings.Contains(image, ":") || strings.Contains(image, "=:") {
				continue
			}

			var skiploop bool
			for _, imgString := range imgStringsToFilter {
				if strings.Contains(image, imgString) {
					skiploop = true
				}
			}

			for _, regexpPattern := range regexToFilter {
				found, _ := regexp.MatchString(regexpPattern, image)

				if found {
					skiploop = true
				}
			}

			if skiploop {
				continue
			}

			registryPath := docker.RegistryPath(image)
			if registryPath.Repository() == "" {
				continue
			}

			if strings.Contains(registryPath.Repository(), ":") {
				continue
			}

			images = append(images, image)
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
