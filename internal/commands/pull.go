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

func newPullCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:       "pull <source|target>",
		Short:     "Pull the images in the manifest",
		Args:      cobra.OnlyValidArgs,
		ValidArgs: []string{"source", "target"},

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("images", cmd.Flags().Lookup("images")); err != nil {
				return fmt.Errorf("bind images flag: %w", err)
			}

			var origin string
			if len(args) > 0 {
				origin = args[0]
			}

			manifestPath := viper.GetString("manifest")
			if err := runPullCommand(origin, manifestPath); err != nil {
				return fmt.Errorf("pull: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringSliceP("images", "i", []string{}, "List of images to pull (e.g. host.com/repo:v1.0.0)")

	return &cmd
}

func runPullCommand(origin string, manifestPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client, err := docker.NewClient(log.Infof)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	var images map[string]string
	if len(viper.GetStringSlice("images")) > 0 {
		images, err = getImagesFromCommandLine(viper.GetStringSlice("images"))
	} else {
		images, err = getImagesFromManifest(manifestPath, origin)
	}
	if err != nil {
		return fmt.Errorf("get images: %w", err)
	}

	log.Infof("Finding images that need to be pulled ...")

	imagesToPull := make(map[string]string)
	for image, auth := range images {
		exists, err := client.ImageExistsOnHost(ctx, image)
		if err != nil {
			return fmt.Errorf("image host existance: %w", err)
		}

		if !exists {
			imagesToPull[image] = auth
		}
	}

	for image, auth := range imagesToPull {
		log.Infof("Pulling %s", image)
		if err := client.PullImageAndWait(ctx, image, auth); err != nil {
			return fmt.Errorf("pull image and wait: %w", err)
		}
		log.Infof("Pulled %s", image)
	}

	log.Infof("All images have been pulled!")

	return nil
}

func getImagesFromManifest(path string, origin string) (map[string]string, error) {
	imageManifest, err := manifest.Get(path)
	if err != nil {
		return nil, fmt.Errorf("get manifest: %w", err)
	}

	images := make(map[string]string)
	for _, source := range imageManifest.Sources {
		var image string
		var auth string

		var err error
		if origin == "target" {
			image = source.TargetImage()
			auth, err = source.Target.EncodedAuth()
		} else {
			image = source.Image()
			auth, err = source.EncodedAuth()
		}
		if err != nil {
			return nil, fmt.Errorf("get %s auth: %w", origin, err)
		}

		images[image] = auth
	}

	return images, nil
}

func getImagesFromCommandLine(images []string) (map[string]string, error) {
	imgs := make(map[string]string)
	for _, image := range images {
		registryPath := docker.RegistryPath(image)

		auth, err := docker.GetEncodedAuthForHost(registryPath.Host())
		if err != nil {
			return nil, fmt.Errorf("get auth: %w", err)
		}

		imgs[image] = auth
	}

	return imgs, nil
}
