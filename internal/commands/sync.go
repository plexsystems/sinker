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
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
)

func newSyncCommand(logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "sync",
		Short: "Sync images in the manifest to the mirror registry",

		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := runSyncCommand(ctx, logger, "."); err != nil {
				return fmt.Errorf("sync: %w", err)
			}

			logger.Println("Sync operation has completed.")

			return nil
		},
	}

	return &cmd
}

func runSyncCommand(ctx context.Context, logger *log.Logger, path string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("new docker client: %w", err)
	}

	currentManifest, err := getManifest(path)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if err := pullOriginImages(ctx, cli, logger, currentManifest); err != nil {
		return fmt.Errorf("pull source image: %w", err)
	}

	logger.Printf("Tagging images ...")
	for _, image := range currentManifest.Images {
		logger.Printf("Tagging image %s -> %s ...", image.Origin(), image.String())
		if err := cli.ImageTag(ctx, image.Origin(), image.String()); err != nil {
			return fmt.Errorf("tagging image: %w", err)
		}
	}

	for _, image := range currentManifest.Images {
		auth, err := getAuthForOriginRegistry(image.OriginRegistry)
		if err != nil {
			return fmt.Errorf("getting auth: %w", err)
		}

		imageExists, err := checkMirrorImageExistsAtRemote(ctx, cli, image, currentManifest.Mirror, auth)
		if err != nil {
			return fmt.Errorf("checking image exists: %w", err)
		}

		if imageExists {
			logger.Printf("Image %s exists at remote registry. Skipping ...", image.String())
			continue
		}

		if err := pushImageToMirrorAndWait(ctx, logger, cli, image, currentManifest.Mirror, auth); err != nil {
			return fmt.Errorf("pushing image to remote: %w", err)
		}
	}

	return nil
}

func checkMirrorImageExistsAtRemote(ctx context.Context, cli *client.Client, image ContainerImage, mirror Mirror, auth string) (bool, error) {
	_, err := cli.ImagePull(ctx, image.String(), types.ImagePullOptions{
		RegistryAuth: auth,
	})

	var notFoundError errdefs.ErrNotFound
	if errors.As(err, &notFoundError) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("pulling image for existance: %w", err)
	}

	return true, nil
}

func pushImageToMirrorAndWait(ctx context.Context, logger *log.Logger, cli *client.Client, image ContainerImage, mirror Mirror, auth string) error {
	pushDestination := mirror.String() + image.String()
	reader, err := cli.ImagePush(ctx, pushDestination, types.ImagePushOptions{
		RegistryAuth: auth,
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
			return fmt.Errorf("unable to unmarshal: %w", err)
		}

		if errorMessage.Error != "" {
			return fmt.Errorf("pushing image after tag: %s", errorMessage.Error)
		}

		if err := json.Unmarshal(streamBytes, &statusMessage); err != nil {
			return fmt.Errorf("unable to unmarshal: %w", err)
		}

		if statusMessage.Status == "Pushing" {
			break
		}
	}

	if err := waitForMirrorImagePush(ctx, logger, cli, image, mirror, auth); err != nil {
		return fmt.Errorf("waiting for push: %w", err)
	}

	return nil
}

func waitForMirrorImagePush(ctx context.Context, logger *log.Logger, cli *client.Client, image ContainerImage, mirror Mirror, auth string) error {
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		exists, err := checkMirrorImageExistsAtRemote(ctx, cli, image, mirror, auth)
		if err != nil {
			return false, fmt.Errorf("checking remote image: %w", err)
		}

		logger.Printf("Pushing %s ...\n", image)
		return exists, nil
	})
}

func getAuthForOriginRegistry(registry string) (string, error) {
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return "", fmt.Errorf("loading docker config: %w", err)
	}

	if !cfg.ContainsAuth() {
		cfg.CredentialsStore = credentials.DetectDefaultStore(cfg.CredentialsStore)
	}

	if registry == "" {
		registry = "https://index.docker.io/v1/"
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

func pullOriginImages(ctx context.Context, cli *client.Client, logger *log.Logger, manifest ImageManifest) error {
	for _, image := range manifest.Images {
		exists, err := originImageExistsLocally(ctx, cli, image)
		if err != nil {
			return fmt.Errorf("checking local image: %w", err)
		}

		if exists {
			logger.Printf("Image %s exists locally. Skipping ...", image.Origin())
			continue
		}

		if err := pullImageAndWait(ctx, logger, cli, image); err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
	}

	return nil
}

func pullImageAndWait(ctx context.Context, logger *log.Logger, cli *client.Client, image ContainerImage) error {
	reader, err := cli.ImagePull(ctx, image.Origin(), types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}

	if err := waitForImagePull(ctx, logger, cli, image); err != nil {
		return fmt.Errorf("waiting for pull: %w", err)
	}
	reader.Close()

	return nil
}

func waitForImagePull(ctx context.Context, logger *log.Logger, cli *client.Client, image ContainerImage) error {
	return wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		exists, err := originImageExistsLocally(ctx, cli, image)
		if err != nil {
			return false, fmt.Errorf("checking local image: %w", err)
		}

		logger.Printf("Pulling %s ...\n", image)
		return exists, nil
	})
}

func originImageExistsLocally(ctx context.Context, cli *client.Client, image ContainerImage) (bool, error) {
	imageList, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return false, fmt.Errorf("getting image list: %w", err)
	}

	if image.OriginRegistry == "docker.io" {
		image.OriginRegistry = ""
	}

	for _, imageSummary := range imageList {
		for _, repoTag := range imageSummary.RepoTags {
			if repoTag == image.Origin() {
				return true, nil
			}
		}
	}

	return false, nil
}
