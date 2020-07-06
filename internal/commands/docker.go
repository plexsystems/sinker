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

// Client ...
type Client struct {
	DockerClient *client.Client
	Logger       *log.Logger
}

// NewClient ...
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

// PullImage ...
func (c Client) PullImage(ctx context.Context, image string, auth string) error {
	opts := types.ImagePullOptions{
		RegistryAuth: auth,
	}

	reader, err := c.DockerClient.ImagePull(ctx, image, opts)
	if err != nil {
		return fmt.Errorf("image pull: %w", err)
	}

	if err := c.waitForImagePulled(ctx, image); err != nil {
		return fmt.Errorf("wait for source image pull: %w", err)
	}
	reader.Close()

	return nil
}

func (c Client) waitForImagePulled(ctx context.Context, image string) error {
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		c.Logger.Printf("Pulling %s ...", image)

		imageList, err := c.DockerClient.ImageList(ctx, types.ImageListOptions{})
		if err != nil {
			return false, fmt.Errorf("image list: %w", err)
		}

		// When an image is sourced from docker hub, the image tag does
		// not include docker.io (or library) on the local machine
		image = strings.ReplaceAll(image, "docker.io/library/", "")
		image = strings.ReplaceAll(image, "docker.io/", "")

		for _, imageSummary := range imageList {
			for _, localImage := range imageSummary.RepoTags {
				if localImage == image {
					return true, nil
				}
			}
		}

		return false, nil
	})
}

// PushImage ...
func (c Client) PushImage(ctx context.Context, image string, auth string) error {
	reader, err := c.DockerClient.ImagePush(ctx, image, types.ImagePushOptions{
		RegistryAuth: auth,
	})
	if err != nil {
		return fmt.Errorf("pushing image: %w", err)
	}
	defer reader.Close()

	if err := waitForPushEvent(reader); err != nil {
		return fmt.Errorf("waiting for push: %w", err)
	}

	if err := c.waitForImagePushed(ctx, image, auth); err != nil {
		return fmt.Errorf("waiting for push: %w", err)
	}

	return nil
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
			break
		}

		if err := json.Unmarshal(streamBytes, &errorMessage); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}

		if errorMessage.Error != "" {
			return fmt.Errorf("pushing image after tag: %s", errorMessage.Error)
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

func (c Client) waitForImagePushed(ctx context.Context, image string, auth string) error {
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		c.Logger.Printf("Pushing %s ...", image)

		exists, err := c.imageExistsAtRemote(ctx, image, auth)
		if err != nil {
			return false, fmt.Errorf("image existance check: %w", err)
		}

		return exists, nil
	})
}

func (c Client) imageExistsAtRemote(ctx context.Context, image string, auth string) (bool, error) {
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
