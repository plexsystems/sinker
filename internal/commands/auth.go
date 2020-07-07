package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
)

func getEncodedSourceAuth(source SourceImage) (string, error) {
	if source.Auth.Password != "" {
		auth, err := getEncodedAuth(source.Auth)
		if err != nil {
			return "", fmt.Errorf("get encoded auth: %w", err)
		}

		return auth, nil
	}

	auth, err := getEncodedAuthForHost(source.Host)
	if err != nil {
		return "", fmt.Errorf("get encoded auth for host: %w", err)
	}

	return auth, nil
}

func getEncodedTargetAuth(target Target) (string, error) {
	if target.Auth.Password != "" {
		auth, err := getEncodedAuth(target.Auth)
		if err != nil {
			return "", fmt.Errorf("get encoded auth: %w", err)
		}

		return auth, nil
	}

	auth, err := getEncodedAuthForHost(target.Host)
	if err != nil {
		return "", fmt.Errorf("get encoded auth for host: %w", err)
	}

	return auth, nil
}

func getEncodedAuth(auth Auth) (string, error) {
	username := os.Getenv(auth.Username)
	password := os.Getenv(auth.Password)

	authConfig := Auth{
		Username: username,
		Password: password,
	}

	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil

}

func getEncodedAuthForHost(host string) (string, error) {
	if host == "" || host == "docker.io" {
		host = "https://index.docker.io/v1/"
	}

	cfg, err := config.Load(config.Dir())
	if err != nil {
		return "", fmt.Errorf("loading docker config: %w", err)
	}

	if !cfg.ContainsAuth() {
		cfg.CredentialsStore = credentials.DetectDefaultStore(cfg.CredentialsStore)
	}

	authConfig, err := cfg.GetAuthConfig(host)
	if err != nil {
		return "", fmt.Errorf("getting auth config: %w", err)
	}

	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}
