package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// A Client manages the communication with the Docker client.
type Client struct {
	docker  *client.Client
	logInfo func(format string, args ...interface{})
}

// NewClient returns a Docker client configured with the given information logger.
func NewClient(logInfo func(format string, args ...interface{})) (Client, error) {
	retry.DefaultDelay = 5 * time.Second
	retry.DefaultAttempts = 2

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return Client{}, fmt.Errorf("new docker client: %w", err)
	}

	client := Client{
		docker:  dockerClient,
		logInfo: logInfo,
	}

	return client, nil
}

// PushImageAndWait pushes an image and waits for it to finish pushing.
// If an error occurs when pushing an image, the push will be attempted again before failing.
func (c Client) PushImageAndWait(ctx context.Context, image string, auth string) error {
	push := func() error {
		if err := c.tryPushImageAndWait(ctx, image, auth); err != nil {
			return fmt.Errorf("try push image: %w", err)
		}

		return nil
	}

	retryFunc := func(attempts uint, err error) {
		c.logInfo("Unable to push %v (Retrying #%v)", image, attempts+1)
	}

	if err := retry.Do(push, retry.OnRetry(retryFunc)); err != nil {
		return fmt.Errorf("retry: %w", err)
	}

	return nil
}

// PullImageAndWait pulls an image and waits for it to finish pulling.
// If an error occurs when pulling an image, the pull will be attempted again before failing.
func (c Client) PullImageAndWait(ctx context.Context, image string, auth string) error {
	pull := func() error {
		if err := c.tryPullImageAndWait(ctx, image, auth); err != nil {
			return fmt.Errorf("try pull image: %w", err)
		}

		return nil
	}

	retryFunc := func(attempts uint, err error) {
		c.logInfo("Unable to pull %v (Retrying #%v)", image, attempts+1)
	}

	if err := retry.Do(pull, retry.OnRetry(retryFunc)); err != nil {
		return fmt.Errorf("retry: %w", err)
	}

	return nil
}

// ImageExistsOnHost returns true if the image exists on the host machine.
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

// GetAllImagesOnHost gets all of the images and their tags on the host.
func (c Client) GetAllImagesOnHost(ctx context.Context) ([]string, error) {
	summaries, err := c.docker.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	var images []string
	for _, summary := range summaries {
		images = append(images, summary.RepoTags...)
	}

	return images, nil
}

// GetAllDigestsOnHost gets all of the images and their digests on the host.
func (c Client) GetAllDigestsOnHost(ctx context.Context) ([]string, error) {
	summaries, err := c.docker.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	var digests []string
	for _, summary := range summaries {
		digests = append(digests, summary.RepoDigests...)
	}

	return digests, nil
}

// GetTagsForRepository returns all of the tags for a given repository.
func (c Client) GetTagsForRepository(ctx context.Context, host string, repository string) ([]string, error) {
	repoPath := "index.docker.io/" + repository
	if host != "" {
		repoPath = host + "/" + repository
	}

	repo, err := name.NewRepository(repoPath)
	if err != nil {
		return nil, fmt.Errorf("new repo: %w", err)
	}

	tags, err := remote.ListWithContext(ctx, repo, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	return tags, nil
}

// Tag creates a new tag from the given target image that references the source image
func (c Client) Tag(ctx context.Context, sourceImage string, targetImage string) error {
	if err := c.docker.ImageTag(ctx, sourceImage, targetImage); err != nil {
		return fmt.Errorf("tag image: %w", err)
	}

	return nil
}

// ImageExistsAtRemote returns true if the image exists at the remote registry.
func (c Client) ImageExistsAtRemote(ctx context.Context, image string) (bool, error) {
	if hasLatestTag(image) {
		return false, nil
	}

	reference, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return false, fmt.Errorf("parse ref: %w", err)
	}

	if _, err := remote.Get(reference, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {

		// If the error is a transport error, check that the error code is of type MANIFEST_UNKNOWN.
		// This is the expected error if an image does not exist.
		if t, exists := err.(*transport.Error); exists {
			for _, diagnostic := range t.Errors {
				if strings.EqualFold("MANIFEST_UNKNOWN", string(diagnostic.Code)) {
					return false, nil
				}
			}
		}

		// If the error is not a transport error, some other error occured
		// that is unrelated to checking if an image exists and it should be returned.
		return false, fmt.Errorf("get image: %w", err)
	}

	return true, nil
}

type progressDetail struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

type statusLine struct {
	ID             string         `json:"id"`
	Message        string         `json:"status"`
	ProgressDetail progressDetail `json:"progressDetail"`
	ErrorMessage   string         `json:"error"`
}

func (c Client) waitForScannerComplete(clientScanner *bufio.Scanner, image string, command string) error {

	// Read the output of the Docker client until there is nothing left to read.
	// When there is nothing left to read, the underlying operation can be considered complete.
	var scans int
	for clientScanner.Scan() {
		var status statusLine
		if err := json.Unmarshal(clientScanner.Bytes(), &status); err != nil {
			return fmt.Errorf("unmarshal status: %w", err)
		}

		if status.ErrorMessage != "" {
			return fmt.Errorf("returned error: %s", status.ErrorMessage)
		}

		// Serves as makeshift polling to occasionally print the status of the Docker command.
		if scans%25 == 0 && status.ProgressDetail.Total > 0 {
			progress := fmt.Sprintf("Processing %vB of %vB", status.ProgressDetail.Current, status.ProgressDetail.Total)
			c.logInfo("%sing %s (%s)", command, image, progress)
		}

		scans++
	}

	if clientScanner.Err() != nil {
		return fmt.Errorf("scanner: %w", clientScanner.Err())
	}

	return nil
}

func (c Client) tryPullImageAndWait(ctx context.Context, image string, auth string) error {
	opts := types.ImagePullOptions{
		RegistryAuth: auth,
	}
	reader, err := c.docker.ImagePull(ctx, image, opts)
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}

	clientScanner := bufio.NewScanner(reader)
	if err := c.waitForScannerComplete(clientScanner, image, "Pull"); err != nil {
		return fmt.Errorf("wait for scanner: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("close reader: %w", err)
	}

	return nil
}

func (c Client) tryPushImageAndWait(ctx context.Context, image string, auth string) error {
	opts := types.ImagePushOptions{
		RegistryAuth: auth,
	}
	reader, err := c.docker.ImagePush(ctx, image, opts)
	if err != nil {
		return fmt.Errorf("push image: %w", err)
	}

	clientScanner := bufio.NewScanner(reader)
	if err := c.waitForScannerComplete(clientScanner, image, "Push"); err != nil {
		return fmt.Errorf("wait for scanner: %w", err)
	}

	if err := reader.Close(); err != nil {
		return fmt.Errorf("close reader: %w", err)
	}

	return nil
}

func imageExists(image string, images []string) bool {

	// When an image is sourced from docker hub, the image tag does
	// not include docker.io (or library) on the local machine.
	image = strings.ReplaceAll(image, "docker.io/library/", "")
	image = strings.ReplaceAll(image, "docker.io/", "")

	for _, currentImage := range images {
		if strings.EqualFold(currentImage, image) {
			return true
		}
	}

	return false
}

func hasLatestTag(image string) bool {
	if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
		return true
	}

	return false
}
