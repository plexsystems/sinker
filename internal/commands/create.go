package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newCreateCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "create <source>",
		Short: "Create a new image manifest",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("target", cmd.Flags().Lookup("target")); err != nil {
				return fmt.Errorf("bind target flag: %w", err)
			}

			var path string
			if len(args) > 0 {
				path = args[0]
			}

			manifestPath := viper.GetString("manifest")
			if err := runCreateCommand(path, manifestPath); err != nil {
				return fmt.Errorf("create: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("target", "t", "", "The target repository to sync images to (e.g. organization.com/repo)")
	cmd.MarkFlagRequired("target")

	return &cmd
}

func runCreateCommand(path string, manifestPath string) error {
	if _, err := GetManifest(manifestPath); err == nil {
		return errors.New("manifest file already exists")
	}

	var err error
	var manifest Manifest
	if path == "" {
		manifest = NewManifest(viper.GetString("target"))
	} else {
		manifest, err = NewAutodetectManifest(viper.GetString("target"), path)
		if err != nil {
			return fmt.Errorf("new manifest with autodetect: %w", err)
		}
	}

	if err := WriteManifest(manifest, manifestPath); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}
