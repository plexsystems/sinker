package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
)

func getAuthForRegistry(registry Registry) (string, error) {
	if registry.Auth.Password != "" {
		username := os.Getenv(registry.Auth.Username)
		password := os.Getenv(registry.Auth.Password)

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

	auth, err := getAuthForHost(registry.Host)
	if err != nil {
		return "", fmt.Errorf("get auth for host: %w", err)
	}

	return auth, nil
}

func getAuthForHost(host string) (string, error) {
	if host == "" {
		host = "https://index.docker.io/v2/"
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
