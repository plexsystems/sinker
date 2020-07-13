package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	pollInterval = 5 * time.Second
	waitTime     = 5 * time.Minute
)

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

// PullImageAndWait pulls an image and waits for it to finish pulling
func (c Client) PullImageAndWait(ctx context.Context, image string, auth string) error {
	opts := types.ImagePullOptions{
		RegistryAuth: auth,
	}

	reader, err := c.DockerClient.ImagePull(ctx, image, opts)
	if err != nil {
		return fmt.Errorf("image pull: %w", err)
	}
	clientByteReader := bufio.NewReader(reader)

	if err := c.waitForImagePulled(ctx, clientByteReader, image); err != nil {
		return fmt.Errorf("waiting for pull: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("closing reader: %w", err)
	}

	return nil
}

func (c Client) waitForImagePulled(ctx context.Context, clientByteReader *bufio.Reader, image string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		status, err := parseReader(c.Logger, clientByteReader)
		if err != nil {
			return false, fmt.Errorf("reader: %w", err)
		}

		fmt.Println(status)

		if status.Message == "Pull complete" {
			return true, nil
		}

		c.Logger.Printf("Pulling %s (%s) ...", image, getStatusString(status))

		return false, nil
	})
}

// PushImageAndWait pushes an image and waits for it to finish pushing
func (c Client) PushImageAndWait(ctx context.Context, image string, auth string) error {
	reader, err := c.DockerClient.ImagePush(ctx, image, types.ImagePushOptions{
		RegistryAuth: auth,
	})
	if err != nil {
		return fmt.Errorf("pushing image: %w", err)
	}

	if err := waitForPushEvent(reader); err != nil {
		return fmt.Errorf("wait for client: %w", err)
	}

	if err := c.waitForImagePushed(ctx, image, auth); err != nil {
		return fmt.Errorf("wait for client: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("close reader: %w", err)
	}

	return nil
}

func (c Client) waitForImagePushed(ctx context.Context, image string, auth string) error {
	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		c.Logger.Printf("Pushing %s ...", image)

		exists, err := c.ImageExistsAtRemote(ctx, image, auth)
		if err != nil {
			return false, fmt.Errorf("image existance check: %w", err)
		}

		return exists, nil
	})
}

// ListAllImages returns a list of all images and their tags found on the local machine
// example: ubuntu:18.04
func (c Client) ListAllImages(ctx context.Context) ([]string, error) {
	var images []string
	imageList, err := c.DockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	for _, image := range imageList {
		images = append(images, image.RepoTags...)
	}

	return images, nil
}

// ListAllDigests returns a list of all images and their digests found on the local machine
// example: ubuntu@sha256:3235326357dfb65f1781dbc4df3b834546d8bf914e82cce58e6e6b676e23ce8f
func (c Client) ListAllDigests(ctx context.Context) ([]string, error) {
	var images []string
	imageList, err := c.DockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list digests: %w", err)
	}

	for _, image := range imageList {
		images = append(images, image.RepoDigests...)
	}

	return images, nil
}

// DigestExistsOnHost returns true if the digest exists on the host
func (c Client) DigestExistsOnHost(ctx context.Context, digest string) (bool, error) {
	digests, err := c.ListAllDigests(ctx)
	if err != nil {
		return false, fmt.Errorf("list all digests: %w", err)
	}

	for _, currentDigest := range digests {
		if strings.EqualFold(currentDigest, digest) {
			return true, nil
		}
	}

	return false, nil
}

// ImageExistsAtRemote returns true if the image exists at the remote
func (c Client) ImageExistsAtRemote(ctx context.Context, image string, auth string) (bool, error) {
	_, err := c.DockerClient.ImagePull(ctx, image, types.ImagePullOptions{
		RegistryAuth: auth,
	})

	var notFoundError errdefs.ErrNotFound
	if errors.As(err, &notFoundError) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("check image for existance: %w", err)
	}

	return true, nil
}

func waitForPushEvent(reader io.ReadCloser) error {
	type ErrorMessage struct {
		Error string
	}

	type StatusMessage struct {
		Status string
	}

	var errorMessage ErrorMessage
	var statusMessage StatusMessage

	buffIOReader := bufio.NewReader(reader)

	for {
		streamBytes, err := buffIOReader.ReadBytes('\n')
		if err == io.EOF {
			return nil
		}

		if err := json.Unmarshal(streamBytes, &errorMessage); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}

		if errorMessage.Error != "" {
			return fmt.Errorf("returned error: %s", errorMessage.Error)
		}

		if err := json.Unmarshal(streamBytes, &statusMessage); err != nil {
			return fmt.Errorf("unmarshal status: %w", err)
		}

		if statusMessage.Status == "Pushing" {
			break
		}
	}

	return nil
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

func getStatusString(status Status) string {
	const defaultStatusMessage = "Processing"

	if status.ProgressDetail.Total > 0 {
		return fmt.Sprintf("Processing layer %vB of %vB", status.ProgressDetail.Current, status.ProgressDetail.Total)
	}

	if strings.Contains(status.Message, "Pulling from") {
		return "Started"
	}

	if strings.Contains(status.Message, "Pulling fs") {
		return fmt.Sprintf("Processing fs layer (trace ID %v)", status.ID)
	}

	if strings.Contains(status.Message, "Verifying") {
		return "Verifying Checksum"
	}

	return defaultStatusMessage
}

type ErrorMessage struct {
	Error string
}

type Status struct {
	Message        string `json:"status"`
	ID             string
	ProgressDetail ProgressDetail
}

type ProgressDetail struct {
	Current int
	Total   int
}

func imageHasLatestTag(image string) bool {
	if !strings.Contains(image, ":") || strings.Contains(image, ":latest") {
		return true
	}

	return false
}
