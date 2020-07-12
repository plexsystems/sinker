package commands

import "testing"

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

func TestSourceImage_WithoutRepository(t *testing.T) {
	image := SourceImage{
		Host: "source.com",
		Tag:  "v1.0.0",
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
			"source.com:v1.0.0",
			"target.com:v1.0.0",
		},
		{
			image,
			targetWithRepository,
			"source.com:v1.0.0",
			"target.com/bar:v1.0.0",
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

func TestSourceImage_WithRepository(t *testing.T) {
	image := SourceImage{
		Host:       "source.com",
		Repository: "repo",
		Tag:        "v1.0.0",
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
			"source.com/repo:v1.0.0",
			"target.com/repo:v1.0.0",
		},
		{
			image,
			targetWithRepository,
			"source.com/repo:v1.0.0",
			"target.com/bar/repo:v1.0.0",
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

func TestSourceImage_WithNestedRepository(t *testing.T) {
	image := SourceImage{
		Host:       "source.com",
		Repository: "repo/foo",
		Tag:        "v1.0.0",
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
			"source.com/repo/foo:v1.0.0",
			"target.com/repo/foo:v1.0.0",
		},
		{
			image,
			targetWithRepository,
			"source.com/repo/foo:v1.0.0",
			"target.com/bar/repo/foo:v1.0.0",
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

func TestSourceImage_Digest(t *testing.T) {
	image := SourceImage{
		Host:       "source.com",
		Repository: "repo",
		Digest:     "sha256:123",
	}

	target := Target{
		Host: "target.com",
	}

	image.Target = target

	const expectedSource = "source.com/repo@sha256:123"
	if image.String() != expectedSource {
		t.Errorf("unexpected source string. expected %s, actual %s", image.String(), expectedSource)
	}

	const expectedTarget = "target.com/repo:123"
	if image.TargetImage() != expectedTarget {
		t.Errorf("unexpected target string. expected %s, actual %s", image.TargetImage(), expectedTarget)
	}
}
