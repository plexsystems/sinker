package commands

import (
	"testing"
)

func TestNewManifest_NoRepository_EmptyRepository(t *testing.T) {
	const expectedRegistry = "registry.com"
	manifest := NewManifest("registry.com")

	if manifest.Target.Host != expectedRegistry {
		t.Errorf("expected target registry url %s, actual %s", expectedRegistry, manifest.Target.Host)
	}

	if manifest.Target.Repository != "" {
		t.Errorf("expected target registry url to be blank, actual %s", manifest.Target.Host)
	}
}

func TestNewManifest_WithRepository_SetsRepository(t *testing.T) {
	const expectedRegistry = "registry.com"
	const expectedRepository = "repo"
	manifest := NewManifest("registry.com/repo")

	if manifest.Target.Repository != expectedRepository {
		t.Errorf("expected target registry url %s, actual %s", expectedRepository, manifest.Target.Repository)
	}
}

func TestTarget_WithoutRepository(t *testing.T) {
	target := Registry{
		Host: "registry.com",
	}

	const expectedTarget = "registry.com"
	if target.String() != expectedTarget {
		t.Errorf("expected target %s, actual %s", expectedTarget, target.String())
	}
}

func TestTarget_WithRepository(t *testing.T) {
	target := Registry{
		Host:       "registry.com",
		Repository: "repo",
	}

	const expectedTarget = "registry.com/repo"
	if target.String() != expectedTarget {
		t.Errorf("expected target %s, actual %s", expectedTarget, target.String())
	}
}

func TestContainerImage_DockerRegistry(t *testing.T) {
	target := Registry{
		Host: "registry.com",
	}

	targetWithRepository := Registry{
		Host:       "registry.com",
		Repository: "bar",
	}

	libraryImage := ContainerImage{
		Source:  Registry{Host: "docker.io", Repository: "repo"},
		Version: "v1.0.0",
	}

	image := ContainerImage{
		Source:  Registry{Host: "docker.io", Repository: "foo/repo"},
		Version: "v1.0.0",
	}

	testCases := []struct {
		image          ContainerImage
		target         Registry
		expectedSource string
		expectedTarget string
	}{
		{
			libraryImage,
			target,
			"docker.io/library/repo:v1.0.0",
			"registry.com/repo:v1.0.0",
		},
		{
			libraryImage,
			targetWithRepository,
			"docker.io/library/repo:v1.0.0",
			"registry.com/bar/repo:v1.0.0",
		},
		{
			image,
			target,
			"docker.io/foo/repo:v1.0.0",
			"registry.com/foo/repo:v1.0.0",
		},
		{
			image,
			targetWithRepository,
			"docker.io/foo/repo:v1.0.0",
			"registry.com/bar/foo/repo:v1.0.0",
		},
	}

	for _, testCase := range testCases {
		testCase.image.Target = testCase.target

		if testCase.image.SourceImage() != testCase.expectedSource {
			t.Errorf("expected source %s, actual %s", testCase.expectedSource, testCase.image.SourceImage())
		}

		if testCase.image.TargetImage() != testCase.expectedTarget {
			t.Errorf("expected target %s, actual %s", testCase.expectedTarget, testCase.image.TargetImage())
		}
	}
}

func TestContainerImage_NonDockerRegistry(t *testing.T) {
	image := ContainerImage{
		Source:  Registry{Host: "quay.io", Repository: "repo"},
		Version: "v1.0.0",
	}

	imageWithPath := ContainerImage{
		Source:  Registry{Host: "quay.io", Repository: "foo/repo"},
		Version: "v1.0.0",
	}

	target := Registry{
		Host: "registry.com",
	}

	targetWithRepository := Registry{
		Host:       "registry.com",
		Repository: "bar",
	}

	testCases := []struct {
		image          ContainerImage
		target         Registry
		expectedSource string
		expectedTarget string
	}{
		{
			image,
			target,
			"quay.io/repo:v1.0.0",
			"registry.com/repo:v1.0.0",
		},
		{
			image,
			targetWithRepository,
			"quay.io/repo:v1.0.0",
			"registry.com/bar/repo:v1.0.0",
		},
		{
			imageWithPath,
			target,
			"quay.io/foo/repo:v1.0.0",
			"registry.com/foo/repo:v1.0.0",
		},
		{
			imageWithPath,
			targetWithRepository,
			"quay.io/foo/repo:v1.0.0",
			"registry.com/bar/foo/repo:v1.0.0",
		},
	}

	for _, testCase := range testCases {
		testCase.image.Target = testCase.target

		if testCase.image.SourceImage() != testCase.expectedSource {
			t.Errorf("expected source %s, actual %s", testCase.expectedSource, testCase.image.SourceImage())
		}

		if testCase.image.TargetImage() != testCase.expectedTarget {
			t.Errorf("expected target %s, actual %s", testCase.expectedTarget, testCase.image.TargetImage())
		}
	}
}
