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
		Short: "Update an existing manifest",
		Args:  cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("output", cmd.Flags().Lookup("output")); err != nil {
				return fmt.Errorf("bind output flag: %w", err)
			}

			outputPath := viper.GetString("manifest")
			if viper.GetString("output") != "" {
				outputPath = viper.GetString("output")
			}

			sourcePath := args[0]
			manifestPath := viper.GetString("manifest")
			if err := runUpdateCommand(sourcePath, manifestPath, outputPath); err != nil {
				return fmt.Errorf("update: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Path where the updated manifest file will be written to")

	return &cmd
}

func runUpdateCommand(path string, manifestPath string, outputPath string) error {
	currentManifest, err := manifest.Get(manifestPath)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	imageManifest, err := manifest.NewWithAutodetect(currentManifest.Target.Host, currentManifest.Target.Repository, path)
	if err != nil {
		return fmt.Errorf("get new manifest: %w", err)
	}

	for s := range imageManifest.Sources {
		for _, currentSource := range currentManifest.Sources {
			if currentSource.Host != imageManifest.Sources[s].Host {
				continue
			}

			if currentSource.Repository != imageManifest.Sources[s].Repository {
				continue
			}

			// If the target host (or repository) of the source does not match the manifest
			// target host (or repository), it has been modified by the user.
			//
			// To preserve the current settings, set the manifest host and repository values
			// to the ones present in the current manifest.
			if currentSource.Target.Host != currentManifest.Target.Host {
				imageManifest.Sources[s].Target.Host = currentSource.Target.Host
			}
			if currentSource.Target.Repository != currentManifest.Target.Repository {
				imageManifest.Sources[s].Target.Repository = currentSource.Target.Repository
			}

			imageManifest.Sources[s].Auth = currentSource.Auth
		}
	}

	if err := imageManifest.Write(outputPath); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}
