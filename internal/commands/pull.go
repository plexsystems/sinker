package commands

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
)

func newPullCommand(ctx context.Context, logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "pull",
		Short: "Pull the source images found in the image manifest",
		Args:  cobra.OnlyValidArgs,

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runPullCommand(ctx, logger); err != nil {
				return fmt.Errorf("pull: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runPullCommand(ctx context.Context, logger *log.Logger) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("new docker client: %w", err)
	}

	manifest, err := GetManifest()
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if len(manifest.Images) == 0 {
		return errors.New("no images found in the image manifest")
	}

	if err := pullSourceImages(ctx, cli, logger, manifest.Images); err != nil {
		return fmt.Errorf("pull source images: %w", err)
	}

	return nil
}

func imageExistsLocally(ctx context.Context, cli *client.Client, image ContainerImage) (bool, error) {
	imageList, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return false, fmt.Errorf("image list: %w", err)
	}

	// When an image is sourced from docker hub, the image tag does
	// not include docker.io on the local machine
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
