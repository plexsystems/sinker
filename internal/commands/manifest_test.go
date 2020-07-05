package commands

import (
	"testing"
)

func TestNewManifest_NoRepository_EmptyRepository(t *testing.T) {
	const expectedRegistry = "registry.com"
	manifest := NewManifest("registry.com")

	if manifest.Target.Registry != expectedRegistry {
		t.Errorf("expected target registry url %s, actual %s", expectedRegistry, manifest.Target.Registry)
	}

	if manifest.Target.Repository != "" {
		t.Errorf("expected target registry url to be blank, actual %s", manifest.Target.Registry)
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
	target := Target{
		Registry: "registry.com",
	}

	const expectedTarget = "registry.com"
	if target.String() != expectedTarget {
		t.Errorf("expected target %s, actual %s", expectedTarget, target.String())
	}
}

func TestTarget_WithRepository(t *testing.T) {
	target := Target{
		Registry:   "registry.com",
		Repository: "repo",
	}

	const expectedTarget = "registry.com/repo"
	if target.String() != expectedTarget {
		t.Errorf("expected target %s, actual %s", expectedTarget, target.String())
	}
}

func TestContainerImage_DockerRegistry(t *testing.T) {
	libraryImage := ContainerImage{
		Repository:     "repo",
		Version:        "v1.0.0",
		SourceRegistry: "docker.io",
	}

	image := ContainerImage{
		Repository:     "foo/repo",
		Version:        "v1.0.0",
		SourceRegistry: "docker.io",
	}

	target := Target{
		Registry: "registry.com",
	}

	targetWithRepository := Target{
		Registry:   "registry.com",
		Repository: "bar",
	}

	testCases := []struct {
		image          ContainerImage
		target         Target
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
		if testCase.image.Source() != testCase.expectedSource {
			t.Errorf("expected source %s, actual %s", testCase.expectedSource, testCase.image.Source())
		}

		if testCase.image.Target(testCase.target) != testCase.expectedTarget {
			t.Errorf("expected target %s, actual %s", testCase.expectedTarget, testCase.image.Target(testCase.target))
		}
	}
}

func TestContainerImage_NonDockerRegistry(t *testing.T) {
	image := ContainerImage{
		Repository:     "repo",
		Version:        "v1.0.0",
		SourceRegistry: "quay.io",
	}

	imageWithPath := ContainerImage{
		Repository:     "foo/repo",
		Version:        "v1.0.0",
		SourceRegistry: "quay.io",
	}

	target := Target{
		Registry: "registry.com",
	}

	targetWithRepository := Target{
		Registry:   "registry.com",
		Repository: "bar",
	}

	testCases := []struct {
		image          ContainerImage
		target         Target
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
		if testCase.image.Source() != testCase.expectedSource {
			t.Errorf("expected source %s, actual %s", testCase.expectedSource, testCase.image.Source())
		}

		if testCase.image.Target(testCase.target) != testCase.expectedTarget {
			t.Errorf("expected target %s, actual %s", testCase.expectedTarget, testCase.image.Target(testCase.target))
		}
	}
}
