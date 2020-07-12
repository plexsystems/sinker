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

	var exists bool
	images, err := c.ListAllImages(ctx)
	if err != nil {
		return fmt.Errorf("list all tags: %w", err)
	}

	for _, currentImage := range images {
		if strings.EqualFold(currentImage, image) {
			exists = true
		}
	}

	if exists {
		return nil
	}

	reader, err := c.DockerClient.ImagePull(ctx, image, opts)
	if err != nil {
		return fmt.Errorf("image pull: %w", err)
	}

	if err := c.waitForClientFinishedReading(ctx, reader, image, auth, "Pulling"); err != nil {
		return fmt.Errorf("waiting for push: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("closing reader: %w", err)
	}

	return nil
}

// PushImageAndWait pushes an image and waits for it to finish pushing
func (c Client) PushImageAndWait(ctx context.Context, image string, auth string) error {
	reader, err := c.DockerClient.ImagePush(ctx, image, types.ImagePushOptions{
		RegistryAuth: auth,
	})
	if err != nil {
		return fmt.Errorf("pushing image: %w", err)
	}

	if err := c.waitForClientFinishedReading(ctx, reader, image, auth, "Pushing"); err != nil {
		return fmt.Errorf("waiting for push: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("closing reader: %w", err)
	}

	return nil
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

func (c Client) waitForClientFinishedReading(ctx context.Context, reader io.ReadCloser, image string, auth string, message string) error {
	return wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
		finished, err := clientFinishedReading(reader)
		if err != nil {
			return false, fmt.Errorf("streaming: %w", err)
		}

		if finished {
			return true, nil
		}

		c.Logger.Printf("%s %s ...", message, image)

		return false, nil
	})
}

func clientFinishedReading(reader io.ReadCloser) (bool, error) {
	type ErrorMessage struct {
		Error string
	}

	type ProgressDetail struct {
		Current int
		Total   int
	}

	type StatusMessage struct {
		Status         string
		ProgressDetail ProgressDetail
	}

	var errorMessage ErrorMessage
	var statusMessage StatusMessage

	buffIOReader := bufio.NewReader(reader)
	streamBytes, err := buffIOReader.ReadBytes('\n')
	if err == io.EOF {
		return true, nil
	}

	if err := json.Unmarshal(streamBytes, &errorMessage); err != nil {
		return false, fmt.Errorf("unmarshal error: %w", err)
	}

	if errorMessage.Error != "" {
		return false, fmt.Errorf("returned error: %s", errorMessage.Error)
	}

	if err := json.Unmarshal(streamBytes, &statusMessage); err != nil {
		return false, fmt.Errorf("unmarshal status: %w", err)
	}

	return false, nil
}
