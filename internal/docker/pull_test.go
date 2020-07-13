package docker

import "testing"

func TestShouldPullImage_Latest(t *testing.T) {
	const image = "docker.io/library/repo:latest"
	imagesOnHost := []string{"repo:latest"}

	shouldPull := shouldPullImage(image, imagesOnHost)

	if !shouldPull {
		t.Error("latest image should always pull, but was not pulled.")
	}
}

func TestShouldPullImage_MissingImage(t *testing.T) {
	const image = "repo:v0.1.0"
	imagesOnHost := []string{"repo:v0.1.0"}

	shouldPull := shouldPullImage(image, imagesOnHost)

	if shouldPull {
		t.Error("image that does not exist on host should pull, but was not.")
	}
}
