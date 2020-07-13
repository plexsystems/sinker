package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// PushImageAndWait pushes an image and waits for it to finish pushing
func (c Client) PushImageAndWait(ctx context.Context, image string, auth string) error {
	reader, err := c.DockerClient.ImagePush(ctx, image, types.ImagePushOptions{
		RegistryAuth: auth,
	})
	if err != nil {
		return fmt.Errorf("pushing image: %w", err)
	}
	/*
		if err := waitForPushEvent(reader); err != nil {
			return fmt.Errorf("wait for client: %w", err)
		}*/

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
