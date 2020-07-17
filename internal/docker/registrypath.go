package docker

import "strings"

// RegistryPath is a registry path for a container image.
type RegistryPath string

// Digest returns the digest in the registry path.
func (r RegistryPath) Digest() string {
	if !strings.Contains(string(r), "@") {
		return ""
	}

	digestTokens := strings.Split(string(r), "@")
	return digestTokens[1]
}

// Tag returns the tag in the registry path.
func (r RegistryPath) Tag() string {
	if strings.Contains(string(r), "@") || !strings.Contains(string(r), ":") {
		return ""
	}

	tagTokens := strings.Split(string(r), ":")
	return tagTokens[1]
}

// Host returns the host in the registry path.
func (r RegistryPath) Host() string {
	host := string(r)

	if r.Tag() != "" {
		host = strings.ReplaceAll(host, ":"+r.Tag(), "")
	}

	if !strings.Contains(host, ".") {
		return ""
	}

	hostTokens := strings.Split(string(r), "/")
	return hostTokens[0]
}

// Repository is the repository in the registry path.
func (r RegistryPath) Repository() string {
	repository := string(r)

	if r.Tag() != "" {
		repository = strings.ReplaceAll(repository, ":"+r.Tag(), "")
	}

	if r.Digest() != "" {
		repository = strings.ReplaceAll(repository, "@"+r.Digest(), "")
	}

	if r.Host() != "" {
		repository = strings.ReplaceAll(repository, r.Host(), "")
	}

	repository = strings.TrimLeft(repository, "/")
	return repository
}
