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

			manifestDirectory := viper.GetString("manifest")
			if err := runPushCommand(ctx, logger, manifestDirectory); err != nil {
				return fmt.Errorf("push: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("dryrun", false, "Print a list of images that would be pushed to the target")

	return &cmd
}

func runPushCommand(ctx context.Context, logger *log.Logger, directory string) error {
	client, err := NewClient(logger)
	if err != nil {
		return fmt.Errorf("new docker client: %w", err)
	}

	manifest, err := GetManifest(directory)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if len(manifest.Images) == 0 {
		return errors.New("no images found in the image manifest")
	}

	logger.Println("Finding images that do not exist at target registry ...")

	var unsyncedImages []SourceImage
	for _, image := range manifest.Images {
		auth, err := getEncodedTargetAuth(image.Target)
		if err != nil {
			return fmt.Errorf("get host auth: %w", err)
		}

		exists, err := client.ImageExistsAtRemote(ctx, image.TargetImage(), auth)
		if err != nil {
			return fmt.Errorf("checking remote target image: %w", err)
		}

		if !exists {
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
		auth, err := getEncodedSourceAuth(image)
		if err != nil {
			return fmt.Errorf("get host auth: %w", err)
		}

		if err := client.PullImageAndWait(ctx, image.String(), auth); err != nil {
			return fmt.Errorf("pulling image: %w", err)
		}
	}

	for _, image := range unsyncedImages {
		if err := client.DockerClient.ImageTag(ctx, image.String(), image.TargetImage()); err != nil {
			return fmt.Errorf("tagging image: %w", err)
		}
	}

	for _, image := range unsyncedImages {
		auth, err := getEncodedTargetAuth(image.Target)
		if err != nil {
			return fmt.Errorf("get source auth: %w", err)
		}

		if err := client.PushImageAndWait(ctx, image.TargetImage(), auth); err != nil {
			return fmt.Errorf("pushing image to target: %w", err)
		}
	}

	client.Logger.Println("All images have been pushed!")

	return nil
}
