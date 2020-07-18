package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/plexsystems/sinker/internal/docker"
	"github.com/plexsystems/sinker/internal/manifest"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newPushCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "push",
		Short: "Push the images in the manifest to the target repository",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("dryrun", cmd.Flags().Lookup("dryrun")); err != nil {
				return fmt.Errorf("bind dryrun flag: %w", err)
			}

			if err := viper.BindPFlag("images", cmd.Flags().Lookup("images")); err != nil {
				return fmt.Errorf("bind images flag: %w", err)
			}

			if err := viper.BindPFlag("target", cmd.Flags().Lookup("target")); err != nil {
				return fmt.Errorf("bind target flag: %w", err)
			}

			if len(viper.GetStringSlice("images")) > 0 && viper.GetString("target") == "" {
				return errors.New("target must be specified when using the images flag")
			}

			manifestPath := viper.GetString("manifest")
			if err := runPushCommand(manifestPath); err != nil {
				return fmt.Errorf("push: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("dryrun", false, "Print a list of images that would be pushed to the target")
	cmd.Flags().StringSliceP("images", "i", []string{}, "List of images to push to target")
	cmd.Flags().StringP("target", "t", "", "Registry the images will be pushed to")

	return &cmd
}

func runPushCommand(manifestPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client, err := docker.NewClient(log.Infof)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	sources := manifest.GetSourcesFromImages(viper.GetStringSlice("images"), viper.GetString("target"))
	if len(sources) == 0 {
		imageManifest, err := manifest.Get(viper.GetString("manifest"))
		if err != nil {
			return fmt.Errorf("get manifest: %w", err)
		}

		sources = imageManifest.Sources
	}

	log.Infof("Finding images that need to be pushed ...")

	var sourcesToPush []manifest.Source
	for _, source := range sources {
		exists, err := client.ImageExistsAtRemote(ctx, source.TargetImage())
		if err != nil {
			return fmt.Errorf("image exists at remote: %w", err)
		}

		if !exists {
			sourcesToPush = append(sourcesToPush, source)
		}
	}

	if len(sourcesToPush) == 0 {
		log.Infof("All images are up to date!")
		return nil
	}

	if viper.GetBool("dryrun") {
		for _, source := range sourcesToPush {
			log.Infof("Image %s would be pushed as %s", source.Image(), source.TargetImage())
		}
		return nil
	}

	for _, source := range sourcesToPush {
		exists, err := client.ImageExistsOnHost(ctx, source.Image())
		if err != nil {
			return fmt.Errorf("image exists: %w", err)
		}

		if !exists {
			sourceAuth, err := source.EncodedAuth()
			if err != nil {
				return fmt.Errorf("get source auth: %w", err)
			}

			log.Infof("Pulling %s", source.Image())
			if err := client.PullImageAndWait(ctx, source.Image(), sourceAuth); err != nil {
				return fmt.Errorf("pull image and wait: %w", err)
			}
			log.Infof("Pulled %s", source.Image())

			if err := client.Tag(ctx, source.Image(), source.TargetImage()); err != nil {
				return fmt.Errorf("tag image: %w", err)
			}
		}

		targetAuth, err := source.Target.EncodedAuth()
		if err != nil {
			return fmt.Errorf("get target auth: %w", err)
		}

		log.Infof("Pushing %s", source.TargetImage())
		if err := client.PushImageAndWait(ctx, source.TargetImage(), targetAuth); err != nil {
			return fmt.Errorf("push image and wait: %w", err)
		}
		log.Infof("Pushed %s", source.TargetImage())
	}

	log.Infof("All images have been pushed!")

	return nil
}
