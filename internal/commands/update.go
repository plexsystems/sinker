package commands

import (
	"fmt"

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
	currentManifest, err := GetManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	updatedManifest, err := NewAutodetectManifest(currentManifest.Target.String(), path)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	for i := range updatedManifest.Images {
		for _, currentImage := range currentManifest.Images {
			if currentImage.Repository != updatedManifest.Images[i].Repository || currentImage.Host != updatedManifest.Images[i].Host {
				continue
			}

			updatedManifest.Images[i].Auth = currentImage.Auth

			if currentManifest.Target.String() != "" {
				updatedManifest.Target = currentImage.Target
			}
		}
	}

	if err := WriteManifest(updatedManifest, manifestPath); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}
