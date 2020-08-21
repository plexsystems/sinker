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
