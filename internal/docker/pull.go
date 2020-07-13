package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// PullImageAndWait pulls an image and waits for it to finish pulling
func (c Client) PullImageAndWait(ctx context.Context, image string, auth string) error {
	opts := types.ImagePullOptions{
		RegistryAuth: auth,
	}

	images, err := c.ListAllImagesWithTags(ctx)
	if err != nil {
		return fmt.Errorf("listing images: %w", err)
	}

	if !shouldPullImage(image, images) {
		return nil
	}

	reader, err := c.DockerClient.ImagePull(ctx, image, opts)
	if err != nil {
		return fmt.Errorf("image pull: %w", err)
	}

	if err := c.waitForImagePulled(ctx, reader, image); err != nil {
		return fmt.Errorf("waiting for pull: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("closing reader: %w", err)
	}

	return nil
}

func (c Client) waitForImagePulled(ctx context.Context, reader io.ReadCloser, image string) error {
	const pullComplete = "Pull complete"
	clientByteReader := bufio.NewReader(reader)

	return wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
		status, err := parseReader(c.Logger, clientByteReader)
		if err != nil {
			return false, fmt.Errorf("reader: %w", err)
		}

		if status.Message == pullComplete {
			return true, nil
		}

		c.Logger.Printf("Pulling %s (%s) ...", image, status.GetMessage())

		return false, nil
	})
}

func shouldPullImage(image string, images []string) bool {
	if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
		return true
	}

	// When an image is sourced from docker hub, the image tag does
	// not include docker.io (or library) on the local machine
	image = strings.ReplaceAll(image, "docker.io/library/", "")
	image = strings.ReplaceAll(image, "docker.io/", "")

	for _, currentImage := range images {
		if strings.EqualFold(currentImage, image) {
			return false
		}
	}

	return true
}
