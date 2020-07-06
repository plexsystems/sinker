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
		Short:     "Pull the source images found in the image manifest",
		Args:      cobra.OnlyValidArgs,
		ValidArgs: []string{"source", "target"},

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runPullCommand(ctx, logger, args[0]); err != nil {
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
		auth, err := getAuthForRegistry(image.Source)
		if err != nil {
			return fmt.Errorf("get auth: %w", err)
		}

		if err := client.PullImage(ctx, image.SourceImage(), auth); err != nil {
			return fmt.Errorf("pull image: %w", err)
		}
	}

	return nil
}
