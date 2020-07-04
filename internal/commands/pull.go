package commands

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
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

	manifest, err := getManifest()
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if len(manifest.Images) == 0 {
		return fmt.Errorf("no images found in manifest (%s)", manifestFileName)
	}

	if err := pullSourceImages(ctx, cli, logger, manifest.Images); err != nil {
		return fmt.Errorf("pull source images: %w", err)
	}

	return nil
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
