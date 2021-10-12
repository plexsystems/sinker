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

func TestGetImagesFromContainers_WithIPParameter(t *testing.T) {
	containers := []corev1.Container{
		{
			Image: "baseimage:v1",
			Args: []string{
				"--serving-address=0.0.0.0:6443",
			},
		},
	}

	actual := getImagesFromContainers(containers)

	if !contains(actual, "baseimage:v1") {
		t.Errorf("expected baseimage:v1 to exist in list of images but it did not: %v", actual)
	}

	if contains(actual, "0.0.0.0:6443") {
		t.Errorf("Invalid image parsing for args contain ip addresses: %v", actual)
	}

}
