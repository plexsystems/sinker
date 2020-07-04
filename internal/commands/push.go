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
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/wait"
)

func newPushCommand(ctx context.Context, logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "push",
		Short: "Push images in the manifest to the target repository",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("dryrun", cmd.Flags().Lookup("dryrun")); err != nil {
				return fmt.Errorf("bind target flag: %w", err)
			}

			if err := runPushCommand(ctx, logger, "."); err != nil {
				return fmt.Errorf("push: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("dryrun", false, "Print a list of images that would be synced during a push")

	return &cmd
}

func runPushCommand(ctx context.Context, logger *log.Logger, path string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("new docker client: %w", err)
	}

	manifest, err := getManifest(path)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if len(manifest.Images) == 0 {
		return fmt.Errorf("no images found in manifest (%s)", manifestFileName)
	}

	var unsyncedImages []ContainerImage
	logger.Println("Finding images that do not exist at target registry ...")
	for _, image := range manifest.Images {
		exists, err := imageExistsAtTarget(ctx, cli, image, manifest.Target)
		if err != nil {
			return fmt.Errorf("checking remote target image: %w", err)
		}

		if !exists {
			logger.Printf("Image %s needs to be synced", image)
			unsyncedImages = append(unsyncedImages, image)
		}
	}

	if len(unsyncedImages) == 0 {
		logger.Println("All images are up to date! 0 images pushed.")
		return nil
	}

	if viper.GetBool("dryrun") {
		for _, image := range unsyncedImages {
			logger.Printf("Image %s would be pushed as %s", image.Source(), image.Target(manifest.Target))
		}
		return nil
	}

	if err := pullSourceImages(ctx, cli, logger, unsyncedImages); err != nil {
		return fmt.Errorf("pull source images: %w", err)
	}

	for _, image := range unsyncedImages {
		if err := cli.ImageTag(ctx, image.Source(), image.Target(manifest.Target)); err != nil {
			return fmt.Errorf("tagging image: %w", err)
		}
	}

	for _, image := range unsyncedImages {
		if err := pushImageToTargetAndWait(ctx, logger, cli, image, manifest.Target); err != nil {
			return fmt.Errorf("pushing image to mirror: %w", err)
		}
	}

	logger.Println("All images have been pushed.")

	return nil
}

func imageExistsAtTarget(ctx context.Context, cli *client.Client, image ContainerImage, target Target) (bool, error) {
	encodedAuth, err := getEncodedAuthForRegistry(target.Registry)
	if err != nil {
		return false, fmt.Errorf("get encoded auth: %w", err)
	}

	_, err = cli.ImagePull(ctx, image.Target(target), types.ImagePullOptions{
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
	encodedAuth, err := getEncodedAuthForRegistry(target.Registry)
	if err != nil {
		return fmt.Errorf("get encoded auth: %w", err)
	}

	reader, err := cli.ImagePush(ctx, image.Target(target), types.ImagePushOptions{
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
		exists, err := imageExistsAtTarget(ctx, cli, image, target)
		if err != nil {
			return false, fmt.Errorf("checking image exists: %w", err)
		}

		logger.Printf("Pushing %s ...", image.Target(target))
		return exists, nil
	})
}

func getEncodedAuthForRegistry(registry string) (string, error) {
	if registry == "" {
		registry = "https://index.docker.io/v2/"
	}

	cfg, err := config.Load(config.Dir())
	if err != nil {
		return "", fmt.Errorf("loading docker config: %w", err)
	}

	if !cfg.ContainsAuth() {
		cfg.CredentialsStore = credentials.DetectDefaultStore(cfg.CredentialsStore)
	}

	authConfig, err := cfg.GetAuthConfig(registry)
	if err != nil {
		return "", fmt.Errorf("getting auth config: %w", err)
	}

	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}

func getEncodedImageAuth(image ContainerImage) (string, error) {
	username := os.Getenv(image.Auth.Username)
	password := os.Getenv(image.Auth.Password)

	authConfig := Auth{
		Username: username,
		Password: password,
	}

	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}

func pullSourceImages(ctx context.Context, cli *client.Client, logger *log.Logger, images []ContainerImage) error {
	for _, image := range images {
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
	var encodedAuth string
	var err error
	if image.Auth.Password != "" {
		encodedAuth, err = getEncodedImageAuth(image)
	} else {
		encodedAuth, err = getEncodedAuthForRegistry(image.SourceRegistry)
	}
	if err != nil {
		return fmt.Errorf("get encoded auth: %w", err)
	}

	opts := types.ImagePullOptions{
		RegistryAuth: encodedAuth,
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
