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

			var path string
			if len(args) > 0 {
				path = args[0]
			}

			manifestPath := viper.GetString("manifest")
			if manifestPath == "" {
				manifestPath = viper.GetString("output")
			}

			if err := runCreateCommand(path, manifestPath); err != nil {
				return fmt.Errorf("create: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("target", "t", "", "The target repository to sync images to (e.g. host.com/repo)")
	cmd.MarkFlagRequired("target")

	cmd.Flags().StringP("output", "o", "", "Path where the manifest file will be written to")

	return &cmd
}

func runCreateCommand(path string, manifestPath string) error {
	if _, err := manifest.Get(manifestPath); err == nil {
		return errors.New("manifest file already exists")
	}

	targetPath := docker.RegistryPath(viper.GetString("target"))
	target := manifest.Target{
		Host:       targetPath.Host(),
		Repository: targetPath.Repository(),
	}

	var images []string
	var err error
	if path == "-" {
		images, err = manifest.GetImagesFromStandardIn()
	} else if path != "" {
		images, err = manifest.GetImagesFromKubernetesManifests(path)
	}
	if err != nil {
		return fmt.Errorf("get images: %w", err)
	}

	sources, err := manifest.GetSourcesFromTarget(images, target)
	if err != nil {
		return fmt.Errorf("get sources: %w", err)
	}

	manifest := manifest.Manifest{
		Target:  target,
		Sources: sources,
	}

	if err := manifest.Write(manifestPath); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}
