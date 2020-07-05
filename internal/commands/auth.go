package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
)

func getEncodedAuthForRegistry(registry string) (string, error) {
	if registry == "" {
		registry = "https://index.docker.io/v2/"
	}

	cfg, err := config.Load(config.Dir())
	if err != nil {
		return "", fmt.Errorf("loading docker config: %w", err)
	}

	if !cfg.ContainsAuth() {
		cfg.CredentialsStore = credentials.DetectDefaultStore(cfg.CredentialsStore)
	}

	authConfig, err := cfg.GetAuthConfig(registry)
	if err != nil {
		return "", fmt.Errorf("getting auth config: %w", err)
	}

	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}

func getEncodedImageAuth(image ContainerImage) (string, error) {
	username := os.Getenv(image.Auth.Username)
	password := os.Getenv(image.Auth.Password)

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
