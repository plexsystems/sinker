package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

// GetEncodedBasicAuth encodes a username and password into Base64.
func GetEncodedBasicAuth(username string, password string) (string, error) {
	authConfig := authn.AuthConfig{
		Username: username,
		Password: password,
	}
	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil

}

// GetEncodedAuthForHost returns a Base64 encoded auth for the given host.
func GetEncodedAuthForHost(host string) (string, error) {
	registryReference, err := name.NewRegistry(host, name.WeakValidation)
	if err != nil {
		return "", fmt.Errorf("new registry: %w", err)
	}

	auth, err := authn.DefaultKeychain.Resolve(registryReference)
	if err != nil {
		return "", fmt.Errorf("resolve auth: %w", err)
	}

	authConfig, err := auth.Authorization()
	if err != nil {
		return "", fmt.Errorf("get auth: %w", err)
	}

	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}
