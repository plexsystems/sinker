package commands

import (
	"testing"
)

func TestTarget_NoRepository_EmptyRepository(t *testing.T) {
	const expected = "target.com"

	target := Target{
		Host: expected,
	}

	if target.String() != expected {
		t.Errorf("expected target to be %s, actual %s", expected, target.String())
	}
}

func TestTarget_NoRepository_ReturnsRepository(t *testing.T) {
	const expected = "target.com/repo"
	target := Target{
		Host:       "target.com",
		Repository: "repo",
	}

	if target.String() != expected {
		t.Errorf("expected target to be %s, actual %s", expected, target.String())
	}
}

func TestPath_NoRepository_EmptyRepository(t *testing.T) {
	const expected = "repo"
	path := Path(expected)

	if path.Host() != "" {
		t.Errorf("expected path host to be blank, actual %s", path.Host())
	}

	if path.Repository() != expected {
		t.Errorf("expected registry repository to be blank, actual %s", path.Repository())
	}
}

func TestPath_WithRepository_ReturnsRepository(t *testing.T) {
	const expectedHost = "url.com"
	const expectedRepository = "foo/bar"

	path := Path("url.com/foo/bar")

	if path.Host() != expectedHost {
		t.Errorf("expected path host %s, actual %s", expectedHost, path.Host())
	}

	if path.Repository() != expectedRepository {
		t.Errorf("expected path repository to be %s actual %s", expectedRepository, path.Repository())
	}
}

func TestSourceImage(t *testing.T) {
	image := SourceImage{
		Host:       "quay.io",
		Repository: "repo",
		Tag:        "v1.0.0",
	}

	imageWithPath := SourceImage{
		Host:       "quay.io",
		Repository: "foo/repo",
		Tag:        "v1.0.0",
	}

	imageWithoutTag := SourceImage{
		Host:       "quay.io",
		Repository: "repo",
	}

	target := Target{
		Host: "target.com",
	}

	targetWithRepository := Target{
		Host:       "target.com",
		Repository: "bar",
	}

	testCases := []struct {
		image          SourceImage
		target         Target
		expectedSource string
		expectedTarget string
	}{
		{
			image,
			target,
			"quay.io/repo:v1.0.0",
			"target.com/repo:v1.0.0",
		},
		{
			image,
			targetWithRepository,
			"quay.io/repo:v1.0.0",
			"target.com/bar/repo:v1.0.0",
		},
		{
			imageWithPath,
			target,
			"quay.io/foo/repo:v1.0.0",
			"target.com/foo/repo:v1.0.0",
		},
		{
			imageWithPath,
			targetWithRepository,
			"quay.io/foo/repo:v1.0.0",
			"target.com/bar/foo/repo:v1.0.0",
		},
		{
			imageWithoutTag,
			target,
			"quay.io/repo",
			"target.com/repo",
		},
	}

	for _, testCase := range testCases {
		testCase.image.Target = testCase.target

		if testCase.image.String() != testCase.expectedSource {
			t.Errorf("expected source %s, actual %s", testCase.expectedSource, testCase.image.String())
		}

		if testCase.image.TargetImage() != testCase.expectedTarget {
			t.Errorf("expected target %s, actual %s", testCase.expectedTarget, testCase.image.TargetImage())
		}
	}
}
