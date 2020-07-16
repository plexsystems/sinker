package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

// Logger defines the required methods for loggers
type Logger interface {
	Printf(format string, args ...interface{})
}

// Client is a Docker client with a logger
type Client struct {
	DockerClient *client.Client
	Logger       Logger
}

// NewClientWithLogger returns a new Docker client with logging
func NewClientWithLogger(logger Logger) (Client, error) {
	retry.DefaultDelay = 5 * time.Second
	retry.DefaultAttempts = 3

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

// PullImageAndWait pulls an image and waits for it to finish pulling
func (c Client) PullImageAndWait(ctx context.Context, image string, auth string) error {
	retryError := retry.Do(
		func() error {
			if err := c.tryPullImageAndWait(ctx, image, auth); err != nil {
				return fmt.Errorf("try pull image: %w", err)
			}

			return nil
		},
		retry.OnRetry(func(retryAttempt uint, err error) {
			c.Logger.Printf("[RETRY] Unable to pull %v (Retrying #%v)", image, retryAttempt+1)
		}),
	)

	if retryError != nil {
		return retryError
	}

	return nil
}

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
	imageSummaries, err := c.DockerClient.ImageList(ctx, types.ImageListOptions{})
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
	var imageRepository string
	if host != "" {
		imageRepository = host + "/" + repository
	} else {
		imageRepository = "index.docker.io/" + repository
	}

	repositoryReference, err := name.NewRepository(imageRepository)
	if err != nil {
		return nil, fmt.Errorf("new repo: %w", err)
	}

	tags, err := remote.List(repositoryReference, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	return tags, nil
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

	if _, err := remote.Get(imageReference, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		var transportError *transport.Error
		if !errors.As(err, &transportError) {
			return false, fmt.Errorf("get image: %w", err)
		}

		for _, diagnostic := range transportError.Errors {
			if strings.EqualFold("MANIFEST_UNKNOWN", string(diagnostic.Code)) {
				return false, nil
			}
		}
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

func getStatusMessage(status statusLine) string {
	if strings.Contains(status.Message, "Pulling from") || strings.Contains(status.Message, "The push refers to") {
		return "Started"
	}

	if status.ProgressDetail.Total > 0 {
		return fmt.Sprintf("Processing %vB of %vB", status.ProgressDetail.Current, status.ProgressDetail.Total)
	}

	return "Processing"
}

func waitForScannerComplete(logger Logger, clientScanner *bufio.Scanner, image string, command string) error {
	var status statusLine
	var scans int
	for clientScanner.Scan() {
		if err := json.Unmarshal(clientScanner.Bytes(), &status); err != nil {
			return fmt.Errorf("unmarshal status: %w", err)
		}

		if status.ErrorMessage != "" {
			return fmt.Errorf("returned error: %s", status.ErrorMessage)
		}

		// Serves as makeshift polling to occasionally print the status of the Docker command.
		if scans%25 == 0 {
			logger.Printf("[%s] %s (%s)", command, image, getStatusMessage(status))
		}

		scans++
	}

	if clientScanner.Err() != nil {
		return fmt.Errorf("scanner: %w", clientScanner.Err())
	}

	logger.Printf("[%s] %s complete.", command, image)

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

func hasLatestTag(image string) bool {
	if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
		return true
	}

	return false
}
