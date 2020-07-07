package commands

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"
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
			if err := runPullCommand(ctx, logger, location); err != nil {
				return fmt.Errorf("pull: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runPullCommand(ctx context.Context, logger *log.Logger, location string) error {
	client, err := NewClient(logger)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	manifest, err := GetManifest()
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if len(manifest.Images) == 0 {
		return errors.New("no images found in the image manifest")
	}

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
			return fmt.Errorf("getting %s auth: %w", location, err)
		}

		if err := client.PullImage(ctx, pullImage, auth); err != nil {
			return fmt.Errorf("pull image: %w", err)
		}
	}

	client.Logger.Println("All images pulled!")

	return nil
}
