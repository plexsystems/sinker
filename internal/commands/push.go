package commands

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
)

// RegistryAuth contains authorization information for connecting to a Registry
type RegistryAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Encode encodes the credentials using Base64
func (r RegistryAuth) Encode() (string, error) {
	jsonAuth, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil

}

func newRegistryAuth(registry string) (RegistryAuth, error) {
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return RegistryAuth{}, fmt.Errorf("load docker config: %w", err)
	}

	if !cfg.ContainsAuth() {
		cfg.CredentialsStore = credentials.DetectDefaultStore(cfg.CredentialsStore)
	}

	authConfig, err := cfg.GetAuthConfig(registry)
	if err != nil {
		return RegistryAuth{}, fmt.Errorf("get auth config: %w", err)
	}

	registryAuth := RegistryAuth{
		Username: authConfig.Username,
		Password: authConfig.Password,
	}

	return registryAuth, nil
}

func newPushCommand(ctx context.Context, logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "push",
		Short: "Push images in the manifest to the target repository",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runPushCommand(ctx, logger, "."); err != nil {
				return fmt.Errorf("push: %w", err)
			}

			logger.Println("All images have been pushed.")

			return nil
		},
	}

	return &cmd
}

func runPushCommand(ctx context.Context, logger *log.Logger, path string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("new docker client: %w", err)
	}

	currentManifest, err := getManifest(path)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if len(currentManifest.Images) == 0 {
		return fmt.Errorf("no images found in manifest (%s)", manifestFileName)
	}

	if err := pullSourceImages(ctx, cli, logger, currentManifest); err != nil {
		return fmt.Errorf("pull source images: %w", err)
	}

	logger.Printf("Tagging images ...")

	for _, image := range currentManifest.Images {
		logger.Printf("Tagging %s -> %s", image.Source(), currentManifest.Target.String()+"/"+image.String())
		if err := cli.ImageTag(ctx, image.Source(), currentManifest.Target.String()+"/"+image.String()); err != nil {
			return fmt.Errorf("tagging image: %w", err)
		}
	}

	for _, image := range currentManifest.Images {
		targetImageExists, err := targetImageExistsAtRemote(ctx, cli, image, currentManifest.Target)
		if err != nil {
			return fmt.Errorf("checking image exists: %w", err)
		}

		if targetImageExists {
			logger.Printf("Image %s exists at mirror. Skipping ...", currentManifest.Target.Repository+"/"+image.String())
			continue
		}

		if err := pushImageToTargetAndWait(ctx, logger, cli, image, currentManifest.Target); err != nil {
			return fmt.Errorf("pushing image to mirror: %w", err)
		}
	}

	return nil
}

func targetImageExistsAtRemote(ctx context.Context, cli *client.Client, image ContainerImage, target Target) (bool, error) {
	registryAuth, err := newRegistryAuth(target.Registry)
	if err != nil {
		return false, fmt.Errorf("get auth for origin: %w", err)
	}

	encodedAuth, err := registryAuth.Encode()
	if err != nil {
		return false, fmt.Errorf("encoding auth: %w", err)
	}

	_, err = cli.ImagePull(ctx, target.String()+"/"+image.String(), types.ImagePullOptions{
		RegistryAuth: encodedAuth,
	})

	var notFoundError errdefs.ErrNotFound
	if errors.As(err, &notFoundError) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("check image for existance: %w", err)
	}

	return true, nil
}

func pushImageToTargetAndWait(ctx context.Context, logger *log.Logger, cli *client.Client, image ContainerImage, target Target) error {
	registryAuth, err := newRegistryAuth(target.Registry)
	if err != nil {
		return fmt.Errorf("get auth for origin: %w", err)
	}

	encodedAuth, err := registryAuth.Encode()
	if err != nil {
		return fmt.Errorf("encoding auth: %w", err)
	}

	reader, err := cli.ImagePush(ctx, target.String()+"/"+image.String(), types.ImagePushOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return fmt.Errorf("pushing image: %w", err)
	}
	defer reader.Close()

	// https://github.com/moby/moby/issues/36253
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

	if err := waitForTargetImagePushed(ctx, logger, cli, image, target); err != nil {
		return fmt.Errorf("waiting for push: %w", err)
	}

	return nil
}

func waitForTargetImagePushed(ctx context.Context, logger *log.Logger, cli *client.Client, image ContainerImage, target Target) error {
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		exists, err := targetImageExistsAtRemote(ctx, cli, image, target)
		if err != nil {
			return false, fmt.Errorf("checking remote target image: %w", err)
		}

		logger.Printf("Pushing %s ...\n", target.String()+"/"+image.String())
		return exists, nil
	})
}

func getSourceRegistryAuth(image ContainerImage) (string, error) {
	if image.Auth.Password != "" {
		username := os.Getenv(image.Auth.Username)
		password := os.Getenv(image.Auth.Password)

		registryAuth := RegistryAuth{
			Username: username,
			Password: password,
		}

		encodedAuth, err := registryAuth.Encode()
		if err != nil {
			return "", fmt.Errorf("encoding auth: %w", err)
		}

		return encodedAuth, nil
	}

	var registry string
	if image.SourceRegistry == "" {
		registry = "https://index.docker.io/v2/"
	} else {
		registry = image.SourceRegistry
	}

	registryAuth, err := newRegistryAuth(registry)
	if err != nil {
		return "", fmt.Errorf("get auth from config: %w", err)
	}

	encodedAuth, err := registryAuth.Encode()
	if err != nil {
		return "", fmt.Errorf("encoding auth: %w", err)
	}

	return encodedAuth, nil
}

func pullSourceImages(ctx context.Context, cli *client.Client, logger *log.Logger, manifest ImageManifest) error {
	for _, image := range manifest.Images {
		exists, err := imageExistsLocally(ctx, cli, image)
		if err != nil {
			return fmt.Errorf("checking local image: %w", err)
		}

		if exists {
			logger.Printf("Image %s exists locally. Skipping ...", image.Source())
			continue
		}

		if err := pullSourceImageAndWait(ctx, logger, cli, image); err != nil {
			return fmt.Errorf("waiting for source image pull: %w", err)
		}
	}

	return nil
}

func pullSourceImageAndWait(ctx context.Context, logger *log.Logger, cli *client.Client, image ContainerImage) error {
	auth, err := getSourceRegistryAuth(image)
	if err != nil {
		return fmt.Errorf("get source auth: %w", err)
	}

	opts := types.ImagePullOptions{
		RegistryAuth: auth,
	}

	reader, err := cli.ImagePull(ctx, image.Source(), opts)
	if err != nil {
		return fmt.Errorf("image pull: %w", err)
	}

	if err := waitForImagePulled(ctx, logger, cli, image); err != nil {
		return fmt.Errorf("wait for source image pull: %w", err)
	}
	reader.Close()

	return nil
}

func waitForImagePulled(ctx context.Context, logger *log.Logger, cli *client.Client, image ContainerImage) error {
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		exists, err := imageExistsLocally(ctx, cli, image)
		if err != nil {
			return false, fmt.Errorf("checking local image: %w", err)
		}

		logger.Printf("Pulling %s ...\n", image.Source())
		return exists, nil
	})
}

func imageExistsLocally(ctx context.Context, cli *client.Client, image ContainerImage) (bool, error) {
	imageList, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return false, fmt.Errorf("image list: %w", err)
	}

	// When an image is sourced from docker hub, the image tag does
	// not include docker.io
	if image.SourceRegistry == "docker.io" {
		image.SourceRegistry = ""
	}

	for _, imageSummary := range imageList {
		for _, localImage := range imageSummary.RepoTags {
			if localImage == image.Source() {
				return true, nil
			}
		}
	}

	return false, nil
}
