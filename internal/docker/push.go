package docker

import (
	"bufio"
	"context"
	"fmt"

	"github.com/avast/retry-go"
	"github.com/docker/docker/api/types"
)

// PushImageAndWait pushes an image and waits for it to finish pushing
func (c Client) PushImageAndWait(ctx context.Context, image string, auth string) error {
	retryError := retry.Do(
		func() error {
			if err := c.tryPushImageAndWait(ctx, image, auth); err != nil {
				return fmt.Errorf("try push image: %w", err)
			}

			return nil
		},
		retry.OnRetry(func(retryAttempt uint, err error) {
			c.Logger.Printf("[RETRY] Unable to push %v (Retrying #%v)", image, retryAttempt+1)
		}),
	)

	if retryError != nil {
		return retryError
	}

	return nil
}

func (c Client) tryPushImageAndWait(ctx context.Context, image string, auth string) error {
	opts := types.ImagePushOptions{
		RegistryAuth: auth,
	}

	reader, err := c.DockerClient.ImagePush(ctx, image, opts)
	if err != nil {
		return fmt.Errorf("push image: %w", err)
	}
	clientScanner := bufio.NewScanner(reader)

	if err := waitForScannerComplete(c.Logger, clientScanner, image, "PUSH"); err != nil {
		return fmt.Errorf("wait for scanner: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("close reader: %w", err)
	}

	return nil
}
