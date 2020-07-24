package commands

import (
	"fmt"

	"github.com/plexsystems/sinker/internal/docker"
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

	var updatedImages []string
	if path == "-" {
		updatedImages, err = manifest.GetImagesFromStandardInput()
	} else {
		updatedImages, err = manifest.GetImagesFromKubernetesManifests(path)
	}
	if err != nil {
		return fmt.Errorf("get images: %w", err)
	}

	var updatedSources []manifest.Source
	for _, updatedImage := range updatedImages {
		updatedRegistryPath := docker.RegistryPath(updatedImage)

		updatedSource := manifest.Source{
			Tag:    updatedRegistryPath.Tag(),
			Digest: updatedRegistryPath.Digest(),
		}

		foundSource, exists := findSourceInManifest(currentManifest, updatedImage)
		if !exists {
			updatedSource.Host = manifest.GetSourceHostFromRepository(updatedRegistryPath.Repository())
			updatedSource.Repository = updatedRegistryPath.Repository()
			updatedSources = append(updatedSources, updatedSource)
			fmt.Println(updatedImage + "was not found.")
			continue
		}

		if foundSource.Target.Host != currentManifest.Target.Host {
			updatedSource.Target.Host = foundSource.Target.Host
		}
		if foundSource.Target.Repository != currentManifest.Target.Repository {
			updatedSource.Target.Repository = foundSource.Target.Repository
		}

		updatedSource.Repository = foundSource.Repository
		updatedSource.Host = foundSource.Host
		updatedSource.Auth = foundSource.Auth

		updatedSources = append(updatedSources, updatedSource)
	}

	updatedManifest := manifest.Manifest{
		Target:  currentManifest.Target,
		Sources: updatedSources,
	}
	if err := updatedManifest.Write(outputPath); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func findSourceInManifest(imageManifest manifest.Manifest, image string) (manifest.Source, bool) {
	for _, currentSource := range imageManifest.Sources {
		imagePath := docker.RegistryPath(image)
		sourceImagePath := docker.RegistryPath(currentSource.Image())
		targetImagePath := docker.RegistryPath(currentSource.TargetImage())

		if imagePath.Host() == sourceImagePath.Host() && imagePath.Repository() == sourceImagePath.Repository() {
			return currentSource, true
		}

		if imagePath.Host() == targetImagePath.Host() && imagePath.Repository() == targetImagePath.Repository() {
			return currentSource, true
		}
	}

	return manifest.Source{}, false
}
