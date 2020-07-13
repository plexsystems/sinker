package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/client"
)

var (
	pollInterval = 5 * time.Second
	waitTime     = 5 * time.Minute
)

// ErrorMessage ...
type ErrorMessage struct {
	Error string
}

// Status ...
type Status struct {
	Message        string `json:"status"`
	ID             string
	ProgressDetail ProgressDetail
}

// GetMessage ...
func (s Status) GetMessage() string {
	const defaultStatusMessage = "Processing"

	if s.ProgressDetail.Total > 0 {
		return fmt.Sprintf("Processing layer %vB of %vB", s.ProgressDetail.Current, s.ProgressDetail.Total)
	}

	if strings.Contains(s.Message, "Pulling from") {
		return "Started"
	}

	if strings.Contains(s.Message, "Pulling fs") {
		return fmt.Sprintf("Processing fs layer (trace ID %v)", s.ID)
	}

	if strings.Contains(s.Message, "Verifying") {
		return "Verifying Checksum"
	}

	return defaultStatusMessage
}

// ProgressDetail ...
type ProgressDetail struct {
	Current int
	Total   int
}

// Client is a Docker client with a logger
type Client struct {
	DockerClient *client.Client
	Logger       *log.Logger
}

// NewClient returns a new Docker client
func NewClient(logger *log.Logger) (Client, error) {
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

func parseReader(logger *log.Logger, clientByteReader *bufio.Reader) (Status, error) {
	var errorMessage ErrorMessage
	var status Status

	completeStatus := Status{
		Message: "Pull complete",
	}

	streamBytes, err := clientByteReader.ReadBytes('\n')
	if err == io.EOF {
		return completeStatus, nil
	}

	if err := json.Unmarshal(streamBytes, &status); err != nil {
		return Status{}, fmt.Errorf("unmarshal status: %w", err)
	}

	if err := json.Unmarshal(streamBytes, &errorMessage); err != nil {
		return Status{}, fmt.Errorf("unmarshal error: %w", err)
	}

	if errorMessage.Error != "" {
		return Status{}, fmt.Errorf("returned error: %s", errorMessage.Error)
	}

	return status, nil
}
