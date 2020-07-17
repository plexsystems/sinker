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

	for u := range updatedManifest.Sources {
		for c := range currentManifest.Sources {
			if currentManifest.Sources[c].Repository != updatedManifest.Sources[u].Repository {
				continue
			}

			if currentManifest.Sources[c].Host != updatedManifest.Sources[u].Host {
				continue
			}

			updatedManifest.Sources[u].Auth = currentManifest.Sources[c].Auth
			updatedManifest.Sources[u].Target = currentManifest.Sources[c].Target
		}
	}

	if err := updatedManifest.Write(manifestPath); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}
