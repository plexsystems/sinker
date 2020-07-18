package docker

import "testing"

type registryPathTest struct {
	actualPath         RegistryPath
	expectedHost       string
	expectedRepository string
	expectedTag        string
	expectedDigest     string
}

func TestRegistryPath_Empty(t *testing.T) {
	path := RegistryPath("")

	test := registryPathTest{
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

	test := registryPathTest{
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

	test := registryPathTest{
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

	test := registryPathTest{
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

	test := registryPathTest{
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

	test := registryPathTest{
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

	test := registryPathTest{
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

	test := registryPathTest{
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

	test := registryPathTest{
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

	test := registryPathTest{
		actualPath:         path,
		expectedHost:       "host.com",
		expectedRepository: "repo",
		expectedTag:        "",
		expectedDigest:     "sha256:abc123",
	}

	verifyRegistryPathMethods(t, test)
}

func verifyRegistryPathMethods(t *testing.T, test registryPathTest) {
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
