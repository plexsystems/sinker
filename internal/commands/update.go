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
			if err := viper.BindPFlag("target", cmd.Flags().Lookup("target")); err != nil {
				return fmt.Errorf("bind target flag: %w", err)
			}

			if err := runUpdateCommand(args[0]); err != nil {
				return fmt.Errorf("update: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("target", "t", "", "The target repository to sync images to (e.g. organization.com/repo)")

	return &cmd
}

func runUpdateCommand(path string) error {
	currentManifest, err := GetManifest()
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	var target string
	if viper.GetString("target") == "" {
		target = currentManifest.Target.Path.String()
	} else {
		target = viper.GetString("target")
	}

	updatedManifest, err := NewAutodetectManifest(target, path)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	var updatedImages []SourceImage
	for _, updatedImage := range updatedManifest.Images {
		for _, currentImage := range currentManifest.Images {
			if currentImage.Path.Repository() == updatedImage.Path.Repository() {
				updatedImage.Path = currentImage.Path
				updatedImage.Target = currentImage.Target
				updatedImage.Auth = currentImage.Auth
			}
		}

		updatedImages = append(updatedImages, updatedImage)
	}

	updatedManifest.Images = updatedImages

	if err := WriteManifest(updatedManifest); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}
