package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/plexsystems/sinker/internal/docker"
	"github.com/plexsystems/sinker/internal/manifest"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newPushCommand(ctx context.Context, logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "push",
		Short: "Push images in the manifest to the target repository",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("dryrun", cmd.Flags().Lookup("dryrun")); err != nil {
				return fmt.Errorf("bind target flag: %w", err)
			}

			manifestPath := viper.GetString("manifest")
			if err := runPushCommand(ctx, logger, manifestPath); err != nil {
				return fmt.Errorf("push: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("dryrun", false, "Print a list of images that would be pushed to the target")

	return &cmd
}

func runPushCommand(ctx context.Context, logger *log.Logger, manifestPath string) error {
	client, err := docker.NewClient(logger)
	if err != nil {
		return fmt.Errorf("new docker client: %w", err)
	}

	imageManifest, err := manifest.Get(manifestPath)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if len(imageManifest.Sources) == 0 {
		return errors.New("no sources found in the image manifest")
	}

	logger.Printf("[INFO] Finding images that do not exist at target registry ...")

	var sourcesToPush []manifest.Source
	for _, source := range imageManifest.Sources {
		exists, err := client.ImageExistsAtRemote(ctx, source.TargetImage())
		if err != nil {
			return fmt.Errorf("image exists at remote: %w", err)
		}

		if !exists {
			sourcesToPush = append(sourcesToPush, source)
		}
	}

	if len(sourcesToPush) == 0 {
		logger.Println("[INFO] All sources exist at the remote registry!.")
		return nil
	}

	if viper.GetBool("dryrun") {
		for _, source := range sourcesToPush {
			logger.Printf("[INFO] Image %s would be pushed as %s", source.Image(), source.TargetImage())
		}
		return nil
	}

	for _, source := range sourcesToPush {
		auth, err := source.EncodedAuth()
		if err != nil {
			return fmt.Errorf("get source auth: %w", err)
		}

		if err := client.PullImageAndWait(ctx, source.Image(), auth); err != nil {
			return fmt.Errorf("pull image and wait: %w", err)
		}
	}

	for _, source := range sourcesToPush {
		if err := client.DockerClient.ImageTag(ctx, source.Image(), source.TargetImage()); err != nil {
			return fmt.Errorf("tagging image: %w", err)
		}
	}

	for _, source := range sourcesToPush {
		auth, err := source.Target.EncodedAuth()
		if err != nil {
			return fmt.Errorf("get target auth: %w", err)
		}

		if err := client.PushImageAndWait(ctx, source.TargetImage(), auth); err != nil {
			return fmt.Errorf("pushing image to target: %w", err)
		}
	}

	client.Logger.Printf("[PUSH] All images have been pushed!")

	return nil
}
