package commands

import (
	"fmt"

	"github.com/plexsystems/sinker/internal/docker"
)

func getEncodedSourceAuth(source SourceImage) (string, error) {
	if source.Auth.Password != "" {
		auth, err := docker.GetEncodedBasicAuth(source.Auth.Username, source.Auth.Password)
		if err != nil {
			return "", fmt.Errorf("get encoded auth: %w", err)
		}

		return auth, nil
	}

	authHost := getAuthHostFromRegistryHost(source.Host)
	auth, err := docker.GetEncodedAuthForHost(authHost)
	if err != nil {
		return "", fmt.Errorf("get encoded auth for host: %w", err)
	}

	return auth, nil
}

func getEncodedTargetAuth(target Target) (string, error) {
	if target.Auth.Password != "" {
		auth, err := docker.GetEncodedBasicAuth(target.Auth.Username, target.Auth.Password)
		if err != nil {
			return "", fmt.Errorf("get encoded auth: %w", err)
		}

		return auth, nil
	}

	authHost := getAuthHostFromRegistryHost(target.Host)
	auth, err := docker.GetEncodedAuthForHost(authHost)
	if err != nil {
		return "", fmt.Errorf("get encoded auth for host: %w", err)
	}

	return auth, nil
}

func getAuthHostFromRegistryHost(host string) string {
	if host == "" || host == "docker.io" {
		return "https://index.docker.io/v1/"
	}

	return host
}
