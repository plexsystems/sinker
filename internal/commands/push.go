package commands

import (
	"context"
	"errors"
	"fmt"
	"log"

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
	client, err := NewClient(logger)
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

	logger.Println("Finding images that do not exist at target registry ...")

	auth, err := getAuthForHost(manifest.Target.Path.Host())
	if err != nil {
		return fmt.Errorf("get host auth: %w", err)
	}

	var unsyncedImages []SourceImage
	for _, image := range manifest.Images {
		exists, err := client.imageExistsAtRemote(ctx, image.Target.Path.Host(), auth)
		if err != nil {
			return fmt.Errorf("checking remote target image: %w", err)
		}

		if !exists {
			logger.Printf("Image %s needs to be synced", image.String())
			unsyncedImages = append(unsyncedImages, image)
		}
	}

	if len(unsyncedImages) == 0 {
		logger.Println("All images are up to date! 0 images pushed.")
		return nil
	}

	if viper.GetBool("dryrun") {
		for _, image := range unsyncedImages {
			logger.Printf("Image %s would be pushed as %s", image.String(), image.TargetImage())
		}
		return nil
	}

	for _, image := range unsyncedImages {
		auth, err := getSourceImageAuth(image)
		if err != nil {
			return fmt.Errorf("get host auth: %w", err)
		}

		if err := client.PullImage(ctx, image.String(), auth); err != nil {
			return fmt.Errorf("pullinh image: %w", err)
		}
	}

	for _, image := range unsyncedImages {
		if err := client.DockerClient.ImageTag(ctx, image.String(), image.TargetImage()); err != nil {
			return fmt.Errorf("tagging image: %w", err)
		}
	}

	for _, image := range unsyncedImages {
		auth, err := getSourceImageAuth(image)
		if err != nil {
			return fmt.Errorf("get host auth: %w", err)
		}

		if err := client.PushImage(ctx, image.TargetImage(), auth); err != nil {
			return fmt.Errorf("pushing image to target: %w", err)
		}
	}

	logger.Println("All images have been pushed.")

	return nil
}
