package manifest

import (
	"encoding/base64"
	"os"
	"reflect"
	"testing"
)

func TestSource_WithoutRepository(t *testing.T) {
	source := Source{
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
		source         Source
		target         Target
		expectedSource string
		expectedTarget string
	}{
		{
			source,
			target,
			"source.com:v1.0.0",
			"target.com:v1.0.0",
		},
		{
			source,
			targetWithRepository,
			"source.com:v1.0.0",
			"target.com/bar:v1.0.0",
		},
	}

	for _, testCase := range testCases {
		testCase.source.Target = testCase.target

		if testCase.source.Image() != testCase.expectedSource {
			t.Errorf("expected source %s, actual %s", testCase.expectedSource, testCase.source.Image())
		}

		if testCase.source.TargetImage() != testCase.expectedTarget {
			t.Errorf("expected target %s, actual %s", testCase.expectedTarget, testCase.source.TargetImage())
		}
	}
}

func TestSource_WithRepository(t *testing.T) {
	source := Source{
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
		source         Source
		target         Target
		expectedSource string
		expectedTarget string
	}{
		{
			source,
			target,
			"source.com/repo:v1.0.0",
			"target.com/repo:v1.0.0",
		},
		{
			source,
			targetWithRepository,
			"source.com/repo:v1.0.0",
			"target.com/bar/repo:v1.0.0",
		},
	}

	for _, testCase := range testCases {
		testCase.source.Target = testCase.target

		if testCase.source.Image() != testCase.expectedSource {
			t.Errorf("expected source %s, actual %s", testCase.expectedSource, testCase.source.Image())
		}

		if testCase.source.TargetImage() != testCase.expectedTarget {
			t.Errorf("expected target %s, actual %s", testCase.expectedTarget, testCase.source.TargetImage())
		}
	}
}

func TestSource_WithNestedRepository(t *testing.T) {
	source := Source{
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
		source         Source
		target         Target
		expectedSource string
		expectedTarget string
	}{
		{
			source,
			target,
			"source.com/repo/foo:v1.0.0",
			"target.com/repo/foo:v1.0.0",
		},
		{
			source,
			targetWithRepository,
			"source.com/repo/foo:v1.0.0",
			"target.com/bar/repo/foo:v1.0.0",
		},
	}

	for _, testCase := range testCases {
		testCase.source.Target = testCase.target

		if testCase.source.Image() != testCase.expectedSource {
			t.Errorf("expected source %s, actual %s", testCase.expectedSource, testCase.source.Image())
		}

		if testCase.source.TargetImage() != testCase.expectedTarget {
			t.Errorf("expected target %s, actual %s", testCase.expectedTarget, testCase.source.TargetImage())
		}
	}
}

func TestSource_Digest(t *testing.T) {
	target := Target{
		Host: "target.com",
	}

	source := Source{
		Host:       "source.com",
		Target:     target,
		Repository: "repo",
		Digest:     "sha256:123",
	}

	const expectedSource = "source.com/repo@sha256:123"
	if source.Image() != expectedSource {
		t.Errorf("unexpected source %s, actual %s", expectedSource, source.Image())
	}

	const expectedTarget = "target.com/repo:123"
	if source.TargetImage() != expectedTarget {
		t.Errorf("unexpected target %s, actual %s", expectedTarget, source.TargetImage())
	}
}

func TestGetSourceHostFromRepository(t *testing.T) {
	testCases := []struct {
		input              string
		expectedSourceHost string
	}{
		{
			input:              "coreos",
			expectedSourceHost: "quay.io",
		},
		{
			input:              "open-policy-agent",
			expectedSourceHost: "quay.io",
		},
		{
			input:              "kubernetes-ingress-controller",
			expectedSourceHost: "quay.io",
		},
		{
			input:              "twistlock",
			expectedSourceHost: "registry.twistlock.com",
		},
	}

	for _, testCase := range testCases {
		sourceHost := getSourceHostFromRepository(testCase.input)

		if sourceHost != testCase.expectedSourceHost {
			t.Errorf("expected source host to be %s, actual %s", testCase.expectedSourceHost, sourceHost)
		}
	}
}

func TestSource_AuthFromEnvironment(t *testing.T) {
	auth := Auth{
		Username: "ENV_USER_KEY",
		Password: "ENV_PASS_KEY",
	}
	target := Target{
		Auth: auth,
	}
	source := Source{
		Target: target,
		Auth:   auth,
	}

	expectedAuthJSON := []byte(`{"Username":"ENV_USER_VALUE","Password":"ENV_PASS_VALUE"}`)
	expectedAuth := base64.URLEncoding.EncodeToString(expectedAuthJSON)

	os.Setenv("ENV_USER_KEY", "ENV_USER_VALUE")
	os.Setenv("ENV_PASS_KEY", "ENV_PASS_VALUE")

	actualSourceAuth, err := source.EncodedAuth()
	if err != nil {
		t.Fatal("encoded source auth:", err)
	}
	if actualSourceAuth != expectedAuth {
		t.Errorf("expected source auth %s, actual %s", expectedAuth, actualSourceAuth)
	}

	actualTargetAuth, err := source.Target.EncodedAuth()
	if err != nil {
		t.Fatal("encoded target auth:", err)
	}
	if actualTargetAuth != expectedAuth {
		t.Errorf("expected target auth %s, actual %s", expectedAuth, actualTargetAuth)
	}
}

func TestSource_TargetDoesNotSupportNestedRepositories_SinglePath(t *testing.T) {
	target := Target{
		Host:       "",
		Repository: "targetrepo",
	}

	source := Source{
		Host:       "source.com",
		Target:     target,
		Repository: "nested/sourcerepo",
		Tag:        "v1.0.0",
	}

	const expectedTarget = "targetrepo/sourcerepo:v1.0.0"
	if source.TargetImage() != expectedTarget {
		t.Errorf("unexpected target string. expected %s, actual %s", expectedTarget, source.TargetImage())
	}
}

func TestSource_TargetDoesNotSupportNestedRepositories_MultiplePaths(t *testing.T) {
	target := Target{
		Host:       "",
		Repository: "targetrepo",
	}

	source := Source{
		Host:       "source.com",
		Target:     target,
		Repository: "really/nested/sourcerepo",
		Tag:        "v1.0.0",
	}

	const expectedTarget = "targetrepo/sourcerepo:v1.0.0"
	if source.TargetImage() != expectedTarget {
		t.Errorf("unexpected target string. expected %s, actual %s", expectedTarget, source.TargetImage())
	}
}

func TestManifest_Update(t *testing.T) {
	base := Manifest{
		Target: Target{
			Host:       "mycr.com",
			Repository: "",
		},
	}

	testCases := []struct {
		desc             string
		existingManifest Manifest
		input            []string
		expected         Manifest
	}{
		{
			desc:             "replaces tags with latest version without repository",
			input:            []string{"mycr.com/foo/bar:1.2.3"},
			existingManifest: base,
			expected: Manifest{
				Target: base.Target,
				Sources: []Source{
					{
						Repository: "foo/bar",
						Tag:        "1.2.3",
					},
				},
			},
		},
		{
			desc:  "replaces tags with latest version with repository",
			input: []string{"mycr.com/foo/bar:1.2.3"},
			existingManifest: Manifest{
				Target: Target{
					Host:       base.Target.Host,
					Repository: "foo",
				},
			},
			expected: Manifest{
				Target: Target{
					Host:       "mycr.com",
					Repository: "foo",
				},
				Sources: []Source{
					{
						Repository: "bar",
						Tag:        "1.2.3",
					},
				},
			},
		},
		{
			desc:  "preserves source specific host overrides from manifest",
			input: []string{"myothercr.com/foo/bar:1.2.3"},
			existingManifest: Manifest{
				Target: base.Target,
				Sources: []Source{
					{
						Repository: "foo/bar",
						Tag:        "1.0.0",
						Target: Target{
							Host: "myothercr.com",
						},
					},
				},
			},
			expected: Manifest{
				Target: base.Target,
				Sources: []Source{
					{
						Repository: "foo/bar",
						Tag:        "1.2.3",
						Target: Target{
							Host: "myothercr.com",
						},
					},
				},
			},
		},
		{
			desc:  "omits target with matching host and repository",
			input: []string{"mycr.com/foo/bar:1.2.3"},
			existingManifest: Manifest{
				Target: Target{
					Host:       "mycr.com",
					Repository: "foo",
				},
				Sources: []Source{
					{
						Repository: "bar",
						Tag:        "1.0.0",
						Target: Target{
							Host:       "mycr.com",
							Repository: "foo",
						},
					},
				},
			},
			expected: Manifest{
				Target: Target{
					Host:       "mycr.com",
					Repository: "foo",
				},
				Sources: []Source{
					{
						Repository: "bar",
						Tag:        "1.2.3",
						Target:     Target{},
					},
				},
			},
		},
		{
			desc:  "includes target with matching host but different repository",
			input: []string{"mycr.com/foo/bar:1.2.3"},
			existingManifest: Manifest{
				Target: Target{
					Host:       "mycr.com",
					Repository: "",
				},
				Sources: []Source{
					{
						Repository: "bar",
						Tag:        "1.0.0",
						Target: Target{
							Host:       "mycr.com",
							Repository: "foo",
						},
					},
				},
			},
			expected: Manifest{
				Target: Target{
					Host:       "mycr.com",
					Repository: "",
				},
				Sources: []Source{
					{
						Repository: "bar",
						Tag:        "1.2.3",
						Target: Target{
							Host:       "mycr.com",
							Repository: "foo",
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			result := testCase.existingManifest.Update(testCase.input)
			if !reflect.DeepEqual(result, testCase.expected) {
				t.Errorf("expected '%v' got '%v'", testCase.expected, result)
			}
		})
	}
}
