package commands

import (
	"fmt"

	"github.com/plexsystems/sinker/internal/manifest"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newUpdateCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "update <source>",
		Short: "Update an existing image manifest",
		Args:  cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			sourcePath := args[0]

			manifestPath := viper.GetString("manifest")
			if err := runUpdateCommand(sourcePath, manifestPath); err != nil {
				return fmt.Errorf("update: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runUpdateCommand(path string, manifestPath string) error {
	currentManifest, err := manifest.Get(manifestPath)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	updatedManifest, err := manifest.NewWithAutodetect(currentManifest.Target.Host, currentManifest.Target.Repository, path)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	for i := range updatedManifest.Sources {
		for _, currentImage := range currentManifest.Sources {
			if currentImage.Repository != updatedManifest.Sources[i].Repository || currentImage.Host != updatedManifest.Sources[i].Host {
				continue
			}

			updatedManifest.Sources[i].Auth = currentImage.Auth

			if currentManifest.Target.Host != "" || currentManifest.Target.Repository != "" {
				updatedManifest.Target = currentImage.Target
			}
		}
	}

	if err := updatedManifest.Write(manifestPath); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}
