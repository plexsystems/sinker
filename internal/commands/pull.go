package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/plexsystems/sinker/internal/docker"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newPullCommand(ctx context.Context, logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:       "pull <source|target>",
		Short:     "Pull the source or target images found in the image manifest",
		Args:      cobra.OnlyValidArgs,
		ValidArgs: []string{"source", "target"},

		RunE: func(cmd *cobra.Command, args []string) error {
			var location string
			if len(args) > 0 {
				location = args[0]
			}

			ctx, cancel := context.WithTimeout(ctx, CommandTimeout)
			defer cancel()

			manifestPath := viper.GetString("manifest")
			if err := runPullCommand(ctx, logger, location, manifestPath); err != nil {
				return fmt.Errorf("pull: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runPullCommand(ctx context.Context, logger *log.Logger, location string, manifestPath string) error {
	client, err := docker.NewClient(logger)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	manifest, err := GetManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if len(manifest.Images) == 0 {
		return errors.New("no images found in the image manifest")
	}

	imagesToPull := make(map[string]string)
	for _, image := range manifest.Images {
		var pullImage string
		var auth string
		var err error
		if location == "target" {
			pullImage = image.TargetImage()
			auth, err = getEncodedTargetAuth(image.Target)
		} else {
			pullImage = image.String()
			auth, err = getEncodedSourceAuth(image)
		}
		if err != nil {
			return fmt.Errorf("get %s auth: %w", location, err)
		}

		exists, err := client.ImageExistsOnHost(ctx, pullImage)
		if err != nil {
			return fmt.Errorf("image host existance: %w", err)
		}

		if !exists {
			client.Logger.Printf("[PULL] Image %s is missing and will be pulled.", pullImage)
			imagesToPull[pullImage] = auth
		}
	}

	for image, auth := range imagesToPull {
		if err != nil {
			return fmt.Errorf("getting %s auth: %w", location, err)
		}

		if err := client.PullImageAndWait(ctx, image, auth); err != nil {
			return fmt.Errorf("pull image: %w", err)
		}
	}

	client.Logger.Printf("[PULL] All images have been pulled!")

	return nil
}
