package manifest

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetImagesFromContainers_WithEqualSign(t *testing.T) {
	containers := []corev1.Container{
		{
			Image: "baseimage:v1",
			Args: []string{
				"--arg=argimage:v1",
				"--newline",
				"newlineimage:v1",
			},
		},
	}

	actual := getImagesFromContainers(containers)

	if !contains(actual, "argimage:v1") {
		t.Errorf("expected argimage:v1 to exist in list of images but it did not: %v", actual)
	}

	if !contains(actual, "newlineimage:v1") {
		t.Errorf("expected newlineimage:v1 to exist in list of images but it did not: %v", actual)
	}
}

func TestGetImagesFromContainers_WithURLParameter(t *testing.T) {
	containers := []corev1.Container{
		{
			Image: "baseimage:v1",
			Args: []string{
				"--events-addr=http://service/",
				"--events-addr=https://service/",
			},
		},
	}
	actual := getImagesFromContainers(containers)

	if !contains(actual, "baseimage:v1") {
		t.Errorf("expected baseimage:v1 to exist in list of images but it did not: %v", actual)
	}

	if contains(actual, "http://service/") {
		t.Errorf("Invalid image parsing for args contain http addresses: %v", actual)
	}

	if contains(actual, "https://service/") {
		t.Errorf("Invalid image parsing for args contain https addresses: %v", actual)
	}

}

// Envoy/Istio specify their log levels as component:level (See https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/run-envoy.html#debugging-envoy)
func TestGetImagesFromContainers_WithLogComponent(t *testing.T) {
	containers := []corev1.Container{
		{
			Image: "baseimage:v1",
			Args: []string{
				"--proxyComponentLogLevel=misc:error",
				"--log_output_level=default:info",
			},
		},
	}
	actual := getImagesFromContainers(containers)

	if !contains(actual, "baseimage:v1") {
		t.Errorf("expected baseimage:v1 to exist in list of images but it did not: %v", actual)
	}

	if contains(actual, "misc:error") {
		t.Errorf("Invalid image parsing for args contain misc:error parameter: %v", actual)
	}

	if contains(actual, "default:info") {
		t.Errorf("Invalid image parsing for args contain default:info parameter: %v", actual)
	}

}
