package docker

import (
	"bufio"
	"context"
	"fmt"

	"github.com/avast/retry-go"
	"github.com/docker/docker/api/types"
)

// PullImageAndWait pulls an image and waits for it to finish pulling
func (c Client) PullImageAndWait(ctx context.Context, image string, auth string) error {
	retryError := retry.Do(
		func() error {
			if err := c.tryPullImageAndWait(ctx, image, auth); err != nil {
				return err
			}

			return nil
		},
		retry.OnRetry(func(retryAttempt uint, err error) {
			c.Logger.Printf("[RETRY] Unable to pull %s (Retrying #%v)", image, retryAttempt+1)
		}),
	)

	if retryError != nil {
		return fmt.Errorf("%w", retryError)
	}

	return nil
}

func (c Client) tryPullImageAndWait(ctx context.Context, image string, auth string) error {
	opts := types.ImagePullOptions{
		RegistryAuth: auth,
	}

	reader, err := c.DockerClient.ImagePull(ctx, image, opts)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	clientScanner := bufio.NewScanner(reader)

	if err := waitForScannerComplete(c.Logger, clientScanner, image, "PULL"); err != nil {
		return fmt.Errorf("wait for scanner: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("close reader: %w", err)
	}

	return nil
}
