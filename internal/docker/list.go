package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// ImageExistsOnHost returns true if the image exists on the host machine
func (c Client) ImageExistsOnHost(ctx context.Context, image string) (bool, error) {
	if hasLatestTag(image) {
		return false, nil
	}

	var images []string
	var err error

	if strings.Contains(image, "@") {
		images, err = c.GetAllDigestsOnHost(ctx)
	} else {
		images, err = c.GetAllImagesOnHost(ctx)
	}
	if err != nil {
		return false, fmt.Errorf("get all images: %w", err)
	}

	if imageExists(image, images) {
		return true, nil
	}

	return false, nil
}

func imageExists(image string, images []string) bool {
	// When an image is sourced from docker hub, the image tag does
	// not include docker.io (or library) on the local machine
	image = strings.ReplaceAll(image, "docker.io/library/", "")
	image = strings.ReplaceAll(image, "docker.io/", "")

	for _, currentImage := range images {
		if strings.EqualFold(currentImage, image) {
			return true
		}
	}

	return false
}

// ImageExistsAtRemote returns true if the image exists at the remote registry
func (c Client) ImageExistsAtRemote(ctx context.Context, image string) (bool, error) {
	if hasLatestTag(image) {
		return false, nil
	}

	imageReference, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return false, fmt.Errorf("parse ref: %w", err)
	}

	_, err = remote.Get(imageReference, remote.WithAuthFromKeychain(authn.DefaultKeychain))

	var transportError *transport.Error
	if errors.As(err, &transportError) {
		for _, diagnostic := range transportError.Errors {
			if strings.EqualFold("MANIFEST_UNKNOWN", string(diagnostic.Code)) {
				return false, nil
			}
		}
	}

	if err != nil {
		return false, fmt.Errorf("get image: %w", err)
	}

	return true, nil
}

// GetAllImagesOnHost gets all of the images and their tags on the host
func (c Client) GetAllImagesOnHost(ctx context.Context) ([]string, error) {
	var images []string
	imageSummaries, err := c.DockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	for _, imageSummary := range imageSummaries {
		images = append(images, imageSummary.RepoTags...)
	}

	return images, nil
}

// GetAllDigestsOnHost gets all of the images and their digests on the host
func (c Client) GetAllDigestsOnHost(ctx context.Context) ([]string, error) {
	var digests []string
	imageSummaries, err := c.DockerClient.ImageList(ctx, types.ImageListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	for _, imageSummary := range imageSummaries {
		digests = append(digests, imageSummary.RepoDigests...)
	}

	return digests, nil
}

// GetTagsForRepo returns all of the tags for a given repository
func (c Client) GetTagsForRepo(ctx context.Context, host string, repository string) ([]string, error) {
	repositoryReference, err := getRepositoryFromImage(host, repository)
	if err != nil {
		return nil, fmt.Errorf("get repository reference: %w", err)
	}

	tags, err := remote.List(repositoryReference, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	return tags, nil
}

func getRepositoryFromImage(host string, repository string) (name.Repository, error) {
	var imageRepository string
	if host != "" {
		imageRepository = host + "/" + repository
	} else {
		imageRepository = "index.docker.io/" + repository
	}

	newRepository, err := name.NewRepository(imageRepository)
	if err != nil {
		return name.Repository{}, fmt.Errorf("new repo: %w", err)
	}

	return newRepository, nil
}

func hasLatestTag(image string) bool {
	if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
		return true
	}

	return false
}
