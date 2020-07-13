package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
)

// ListAllImagesWithTags returns a list of all images and their tags found on the local machine
// example: ubuntu:18.04
func (c Client) ListAllImagesWithTags(ctx context.Context) ([]string, error) {
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
