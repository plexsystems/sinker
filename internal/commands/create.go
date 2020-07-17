package commands

import (
	"errors"
	"fmt"

	"github.com/plexsystems/sinker/internal/docker"
	"github.com/plexsystems/sinker/internal/manifest"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newCreateCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "create <source>",
		Short: "Create a new a manifest",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("target", cmd.Flags().Lookup("target")); err != nil {
				return fmt.Errorf("bind target flag: %w", err)
			}

			if err := viper.BindPFlag("output", cmd.Flags().Lookup("output")); err != nil {
				return fmt.Errorf("bind output flag: %w", err)
			}

			var resourcePath string
			if len(args) > 0 {
				resourcePath = args[0]
			}

			manifestPath := viper.GetString("manifest")
			if manifestPath == "" {
				manifestPath = viper.GetString("output")
			}

			if err := runCreateCommand(resourcePath, manifestPath); err != nil {
				return fmt.Errorf("create: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("target", "t", "", "The target repository to sync images to (e.g. host.com/repo)")
	cmd.Flags().StringP("output", "o", "", "Path where the manifest file will be written to")
	cmd.MarkFlagRequired("target")

	return &cmd
}

func runCreateCommand(resourcePath string, manifestPath string) error {
	if _, err := manifest.Get(manifestPath); err == nil {
		return errors.New("manifest file already exists")
	}

	targetPath := docker.RegistryPath(viper.GetString("target"))

	var err error
	var imageManifest manifest.Manifest
	if resourcePath == "" {
		imageManifest = manifest.New(targetPath.Host(), targetPath.Repository())
	} else {
		imageManifest, err = manifest.NewWithAutodetect(targetPath.Host(), targetPath.Repository(), resourcePath)
		if err != nil {
			return fmt.Errorf("new manifest with autodetect: %w", err)
		}
	}

	if err := imageManifest.Write(manifestPath); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}
