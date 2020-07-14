package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

// Client is a Docker client with a logger
type Client struct {
	DockerClient *client.Client
	Logger       *log.Logger
}

// NewClient returns a new Docker client
func NewClient(logger *log.Logger) (Client, error) {
	retry.DefaultDelay = 5 * time.Second
	retry.DefaultAttempts = 3

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return Client{}, fmt.Errorf("new docker client: %w", err)
	}

	client := Client{
		DockerClient: dockerClient,
		Logger:       logger,
	}

	return client, nil
}

// RegistryPath is a registry path for a docker image
type RegistryPath string

// Digest is the digest in the registry path, if one exists
func (r RegistryPath) Digest() string {
	if !strings.Contains(string(r), "@") {
		return ""
	}

	digestTokens := strings.Split(string(r), "@")

	return digestTokens[1]
}

// Tag is the tag in the registry path, if one exists
func (r RegistryPath) Tag() string {
	if strings.Contains(string(r), "@") || !strings.Contains(string(r), ":") {
		return ""
	}

	tagTokens := strings.Split(string(r), ":")

	return tagTokens[1]
}

// Host is the host in the registry path
func (r RegistryPath) Host() string {
	if !strings.Contains(string(r), "/") {
		return string(r)
	}

	hostTokens := strings.Split(string(r), "/")

	return hostTokens[0]
}

// Repository is the repository in the registry path
func (r RegistryPath) Repository() string {
	if !strings.Contains(string(r), "/") {
		return ""
	}

	repository := string(r)

	if r.Tag() != "" {
		repository = strings.ReplaceAll(repository, ":"+r.Tag(), "")
	}

	if r.Digest() != "" {
		repository = strings.ReplaceAll(repository, "@"+r.Digest(), "")
	}

	if r.Host() != "" {
		repository = strings.ReplaceAll(repository, r.Host()+"/", "")
	}

	return repository
}

// ErrorMessage is an error message from the Docker client
type ErrorMessage struct {
	Error string `json:"error"`
}

// Status is the status output from the Docker client
type Status struct {
	Message        string         `json:"status"`
	ID             string         `json:"id"`
	ProgressDetail ProgressDetail `json:"progressDetail"`
}

// ProgressDetail is the current state of pushing or pulling an image (in Bytes)
type ProgressDetail struct {
	Current int
	Total   int
}

// GetMessage returns a human friendly message from parsing the status message
func (s Status) GetMessage() string {
	const defaultStatusMessage = "Processing"

	if strings.Contains(s.Message, "Pulling from") || strings.Contains(s.Message, "The push refers to repository") {
		return "Started"
	}

	if strings.Contains(s.Message, "Pulling fs") || strings.Contains(s.Message, "Layer already exists") {
		return fmt.Sprintf("Processing layer (trace ID %v)", s.ID)
	}

	if s.ProgressDetail.Total > 0 {
		return fmt.Sprintf("Processing %vB of %vB", s.ProgressDetail.Current, s.ProgressDetail.Total)
	}

	if strings.Contains(s.Message, "Preparing") {
		return "Preparing"
	}

	if strings.Contains(s.Message, "Verifying") {
		return "Verifying Checksum"
	}

	return defaultStatusMessage
}

func waitForScannerComplete(logger *log.Logger, clientScanner *bufio.Scanner, image string, command string) error {
	var errorMessage ErrorMessage
	var status Status

	var scans int
	for clientScanner.Scan() {
		if err := json.Unmarshal(clientScanner.Bytes(), &status); err != nil {
			return fmt.Errorf("unmarshal status: %w", err)
		}

		if err := json.Unmarshal(clientScanner.Bytes(), &errorMessage); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}

		if errorMessage.Error != "" {
			return fmt.Errorf("returned error: %s", errorMessage.Error)
		}

		if scans%50 == 0 {
			logger.Printf("[%s] %s (%s)", command, image, status.GetMessage())
		}

		scans++
	}

	if clientScanner.Err() != nil {
		return fmt.Errorf("scanner: %w", clientScanner.Err())
	}

	logger.Printf("[%s] %s complete.", command, image)

	return nil
}
