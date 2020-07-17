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

	cmd.Flags().StringP("output", "o", "", "Path where the updated manifest file will be written to (defaults to the current manifest file)")

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

	for u := range imageManifest.Sources {
		for c := range currentManifest.Sources {
			if currentManifest.Sources[c].Repository != imageManifest.Sources[u].Repository {
				continue
			}

			if currentManifest.Sources[c].Host != imageManifest.Sources[u].Host {
				continue
			}

			// If the target host (or repository) of a source does not match the root
			// target host (or repository), it has been modified by the user.
			//
			// To preserve the current settings and not overwrite them, set the
			// manifest host and repository values to the ones present in the current manifest.
			if currentManifest.Sources[c].Target.Host != currentManifest.Target.Host {
				imageManifest.Sources[u].Target.Host = currentManifest.Sources[c].Target.Host
			}
			if currentManifest.Sources[c].Target.Repository != currentManifest.Target.Repository {
				imageManifest.Sources[u].Target.Repository = currentManifest.Sources[c].Target.Repository
			}

			imageManifest.Sources[u].Auth = currentManifest.Sources[c].Auth
		}
	}

	if err := imageManifest.Write(outputPath); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}
