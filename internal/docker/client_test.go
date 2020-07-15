package docker

import "testing"

type RegistryPathTest struct {
	actualPath         RegistryPath
	expectedHost       string
	expectedRepository string
	expectedTag        string
	expectedDigest     string
}

func verifyRegistryPathMethods(t *testing.T, test RegistryPathTest) {
	if test.actualPath.Host() != test.expectedHost {
		t.Errorf("expected host to be %s, actual %s", test.expectedHost, test.actualPath.Host())
	}

	if test.actualPath.Repository() != test.expectedRepository {
		t.Errorf("expected repository to be %s, actual %s", test.expectedRepository, test.actualPath.Repository())
	}

	if test.actualPath.Tag() != test.expectedTag {
		t.Errorf("expected tag to be %s, actual %s", test.expectedTag, test.actualPath.Tag())
	}

	if test.actualPath.Digest() != test.expectedDigest {
		t.Errorf("expected digest to be %s, actual %s", test.expectedDigest, test.actualPath.Digest())
	}
}

func TestPath_Empty(t *testing.T) {
	const expected = ""
	path := RegistryPath(expected)

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "",
		expectedRepository: "",
		expectedTag:        "",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Host(t *testing.T) {
	path := RegistryPath("host.com")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "host.com",
		expectedRepository: "",
		expectedTag:        "",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Host_WithSlash(t *testing.T) {
	path := RegistryPath("host.com/")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "host.com",
		expectedRepository: "",
		expectedTag:        "",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Repository_NoHost(t *testing.T) {
	path := RegistryPath("repo:v1.0.0")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "",
		expectedRepository: "repo",
		expectedTag:        "v1.0.0",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Repository_RepeatedName(t *testing.T) {
	path := RegistryPath("repo/repository:v1.0.0")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "",
		expectedRepository: "repo/repository",
		expectedTag:        "v1.0.0",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Repository_OneLevel(t *testing.T) {
	path := RegistryPath("host.com/repo")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "host.com",
		expectedRepository: "repo",
		expectedTag:        "",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Repository_MultipleLevels(t *testing.T) {
	path := RegistryPath("host.com/repo/more")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "host.com",
		expectedRepository: "repo/more",
		expectedTag:        "",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Tag(t *testing.T) {
	path := RegistryPath("host.com/repo:v1.0.0")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "host.com",
		expectedRepository: "repo",
		expectedTag:        "v1.0.0",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Tag_None(t *testing.T) {
	path := RegistryPath("host.com/repo")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "host.com",
		expectedRepository: "repo",
		expectedTag:        "",
		expectedDigest:     "",
	}

	verifyRegistryPathMethods(t, test)
}

func TestRegistryPath_Digest(t *testing.T) {
	path := RegistryPath("host.com/repo@sha256:abc123")

	test := RegistryPathTest{
		actualPath:         path,
		expectedHost:       "host.com",
		expectedRepository: "repo",
		expectedTag:        "",
		expectedDigest:     "sha256:abc123",
	}

	verifyRegistryPathMethods(t, test)
}
