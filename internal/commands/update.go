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
			manifestDirectory := viper.GetString("manifest")

			if err := runUpdateCommand(args[0], manifestDirectory); err != nil {
				return fmt.Errorf("update: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runUpdateCommand(path string, directory string) error {
	currentManifest, err := GetManifest(directory)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	updatedManifest, err := NewAutodetectManifest(currentManifest.Target.String(), path)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	for i := range updatedManifest.Images {
		for _, currentImage := range currentManifest.Images {
			if currentImage.Repository == updatedManifest.Images[i].Repository && currentImage.Host == updatedManifest.Images[i].Host {
				updatedManifest.Images[i].Auth = currentImage.Auth

				if currentManifest.Target.String() != "" {
					updatedManifest.Target = currentImage.Target
				}
			}
		}
	}

	if err := writeManifest(updatedManifest, directory); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}
