package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/plexsystems/sinker/internal/docker"
	"github.com/plexsystems/sinker/internal/manifest"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newPullCommand(logger *log.Logger) *cobra.Command {
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

			manifestPath := viper.GetString("manifest")
			if err := runPullCommand(logger, location, manifestPath); err != nil {
				return fmt.Errorf("pull: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runPullCommand(logger *log.Logger, location string, manifestPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client, err := docker.NewClientWithLogger(logger)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	manifest, err := manifest.Get(manifestPath)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	imagesToPull := make(map[string]string)
	for _, source := range manifest.Sources {
		var pullImage string
		var auth string
		var err error
		if location == "target" {
			pullImage = source.TargetImage()
			auth, err = source.Target.EncodedAuth()
		} else {
			pullImage = source.Image()
			auth, err = source.EncodedAuth()
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
		if err := client.PullImageAndWait(ctx, image, auth); err != nil {
			return fmt.Errorf("pull image: %w", err)
		}
	}

	client.Logger.Printf("[PULL] All images have been pulled!")

	return nil
}
