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

	cmd.Flags().StringP("output", "o", "", "Path where the updated manifest file will be written to (defaults to the current manifest file")

	return &cmd
}

func runUpdateCommand(path string, manifestPath string, outputPath string) error {
	currentManifest, err := manifest.Get(manifestPath)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	updatedManifest, err := manifest.NewWithAutodetect(currentManifest.Target.Host, currentManifest.Target.Repository, path)
	if err != nil {
		return fmt.Errorf("get current manifest: %w", err)
	}

	for u := range updatedManifest.Sources {
		for c := range currentManifest.Sources {
			if currentManifest.Sources[c].Repository != updatedManifest.Sources[u].Repository {
				continue
			}

			if currentManifest.Sources[c].Host != updatedManifest.Sources[u].Host {
				continue
			}

			// sdkfjsdkf .....
			updatedManifest.Sources[u].Auth = currentManifest.Sources[c].Auth

			if currentManifest.Sources[c].Target.Host != currentManifest.Target.Host {
				updatedManifest.Sources[u].Target.Host = currentManifest.Sources[c].Target.Host
			}

			if currentManifest.Sources[c].Target.Repository != currentManifest.Target.Repository {
				updatedManifest.Sources[u].Target.Repository = currentManifest.Sources[c].Target.Repository
			}
		}
	}

	if err := updatedManifest.Write(outputPath); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}
