package docker

import "testing"

func TestImageExists_DockerIO(t *testing.T) {
	imagesOnHost := []string{"busybox:1.0.0", "plexsystems/busybox:1.0.0"}
	image := "docker.io/busybox:1.0.0"

	exists := imageExists(image, imagesOnHost)

	if !exists {
		t.Errorf("expected docker.io address to exist, but it did not.")
	}
}

func TestImageExists_DockerIO_WithLibrary(t *testing.T) {
	imagesOnHost := []string{"busybox:1.0.0", "plexsystems/busybox:1.0.0"}
	image := "docker.io/library/busybox:1.0.0"

	exists := imageExists(image, imagesOnHost)

	if !exists {
		t.Errorf("expected docker.io address to exist, but it did not.")
	}
}
