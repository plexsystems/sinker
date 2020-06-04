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
	"path/filepath"
	"time"

	"cuelang.org/go/pkg/strings"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Interval defines the frequency in which to check if a Docker operation has completed
	Interval = 5 * time.Second

	// WaitTime defines how long to wait for Docker to complete the operation
	WaitTime = 5 * time.Minute
)

// NewSyncCommand creates a new sync command
func NewSyncCommand(logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "sync",
		Short: "Sync the images found in the repository to another registry",
		Args:  cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("mirror", cmd.Flags().Lookup("mirror")); err != nil {
				return fmt.Errorf("bind flag: %w", err)
			}

			if err := runSyncCommand(logger, args); err != nil {
				return fmt.Errorf("sync: %w", err)
			}

			logger.Println("Sync operation has completed.")

			return nil
		},
	}

	cmd.Flags().StringP("mirror", "m", "", "mirror prefix")

	return &cmd
}

func runSyncCommand(logger *log.Logger, args []string) error {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("new docker client: %w", err)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working dir: %w", err)
	}

	path := filepath.Join(workingDir, args[0])

	mirrorImages, err := GetImagesFromFile(path)
	if err != nil {
		return fmt.Errorf("get before list: %w", err)
	}

	var originalImages []DockerImage
	for _, mirrorImage := range mirrorImages {
		originalImage := getOriginalImage(mirrorImage, viper.GetString("mirror"))
		originalImages = append(originalImages, originalImage)
	}

	imageMap := getImageMap(originalImages, mirrorImages)

	if err := pullSourceImages(ctx, cli, logger, originalImages); err != nil {
		return fmt.Errorf("pull source image: %w", err)
	}

	logger.Printf("Tagging images...")
	for originalImage, mirrorImage := range imageMap {
		if err := cli.ImageTag(ctx, originalImage.String(), mirrorImage.String()); err != nil {
			return fmt.Errorf("tagging image: %w", err)
		}
	}

	for _, mirrorImage := range mirrorImages {
		auth, err := getAuthForHost(mirrorImage.Host)
		if err != nil {
			return fmt.Errorf("getting auth: %w", err)
		}

		imageExists, err := checkImageExistsAtRemote(ctx, cli, mirrorImage, auth)
		if err != nil {
			return fmt.Errorf("checking image exists: %w", err)
		}

		if imageExists {
			logger.Printf("Image %s exists at remote registry. Skipping...", mirrorImage.String())
			continue
		}

		if err := pushImageAndWait(ctx, logger, cli, mirrorImage, auth); err != nil {
			return fmt.Errorf("pushing image to remote: %w", err)
		}
	}

	return nil
}

func checkImageExistsAtRemote(ctx context.Context, cli *client.Client, image DockerImage, auth string) (bool, error) {
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

func pushImageAndWait(ctx context.Context, logger *log.Logger, cli *client.Client, image DockerImage, auth string) error {
	reader, err := cli.ImagePush(ctx, image.String(), types.ImagePushOptions{
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

	if err := waitForImagePush(ctx, logger, cli, image, auth); err != nil {
		return fmt.Errorf("waiting for push: %w", err)
	}

	return nil
}

func waitForImagePush(ctx context.Context, logger *log.Logger, cli *client.Client, image DockerImage, auth string) error {
	return wait.PollImmediate(Interval, 5*time.Minute, func() (bool, error) {
		exists, err := checkImageExistsAtRemote(ctx, cli, image, auth)
		if err != nil {
			return false, fmt.Errorf("checking remote image: %w", err)
		}

		logger.Printf("Pushing %s...\n", image)
		return exists, nil
	})
}

func getAuthForHost(host string) (string, error) {
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return "", fmt.Errorf("loading docker config: %w", err)
	}

	if !cfg.ContainsAuth() {
		cfg.CredentialsStore = credentials.DetectDefaultStore(cfg.CredentialsStore)
	}

	if host == "" {
		host = "https://index.docker.io/v1/"
	}

	authConfig, err := cfg.GetAuthConfig(host)
	if err != nil {
		return "", fmt.Errorf("getting auth config: %w", err)
	}

	jsonAuth, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshal auth: %w", err)
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}

func pullSourceImages(ctx context.Context, cli *client.Client, logger *log.Logger, sourceImages []DockerImage) error {
	for _, image := range sourceImages {
		exists, err := imageExistsLocally(ctx, cli, image)
		if err != nil {
			return fmt.Errorf("checking local image: %w", err)
		}

		if exists {
			logger.Printf("Image %s exists locally. Skipping...", image.String())
			continue
		}

		if err := pullImageAndWait(ctx, logger, cli, image); err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
	}

	return nil
}

func pullImageAndWait(ctx context.Context, logger *log.Logger, cli *client.Client, image DockerImage) error {
	reader, err := cli.ImagePull(ctx, image.String(), types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image: %w", err)
	}

	if err := waitForImagePull(ctx, logger, cli, image); err != nil {
		return fmt.Errorf("waiting for pull: %w", err)
	}
	reader.Close()

	return nil
}

func waitForImagePull(ctx context.Context, logger *log.Logger, cli *client.Client, image DockerImage) error {
	return wait.PollImmediate(Interval, 5*time.Minute, func() (bool, error) {
		exists, err := imageExistsLocally(ctx, cli, image)
		if err != nil {
			return false, fmt.Errorf("checking local image: %w", err)
		}

		logger.Printf("Pulling %s...\n", image)
		return exists, nil
	})
}

func imageExistsLocally(ctx context.Context, cli *client.Client, image DockerImage) (bool, error) {
	imageList, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return false, fmt.Errorf("getting image list: %w", err)
	}

	if image.Host == "docker.io" {
		image.Host = ""
	}

	for _, imageSummary := range imageList {
		for _, repoTag := range imageSummary.RepoTags {
			if repoTag == image.String() {
				return true, nil
			}
		}
	}

	return false, nil
}

func getImageMap(sourceImages []DockerImage, destinationImages []DockerImage) map[DockerImage]DockerImage {
	imageMap := make(map[DockerImage]DockerImage)

	for _, sourceImage := range sourceImages {
		for _, destinationImage := range destinationImages {
			if strings.Contains(destinationImage.Name, sourceImage.Name) {
				imageMap[sourceImage] = destinationImage
				break
			}
		}
	}

	return imageMap
}
