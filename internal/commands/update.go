package commands

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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
	if _, err := os.Stat(manifestFileName); os.IsNotExist(err) {
		return fmt.Errorf("manifest %s not found in current directory", manifestFileName)
	}

	imageManifest, err := getManifest()
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	if viper.GetString("target") != "" {
		imageManifest.Target = newTarget(viper.GetString("target"))
	}

	foundImages, err := getFromKubernetesManifests(path, imageManifest.Target)
	if err != nil {
		return fmt.Errorf("get from kubernetes manifests: %w", err)
	}

	imageManifest.Images, err = getUpdatedImages(foundImages, imageManifest)
	if err != nil {
		return fmt.Errorf("get updated images: %w", err)
	}

	if err := writeManifest(imageManifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func getManifest() (ImageManifest, error) {
	imageManifestContents, err := ioutil.ReadFile(manifestFileName)
	if err != nil {
		return ImageManifest{}, fmt.Errorf("reading manifest: %w", err)
	}

	var currentImageManifest ImageManifest
	if err := yaml.Unmarshal(imageManifestContents, &currentImageManifest); err != nil {
		return ImageManifest{}, fmt.Errorf("unmarshal current manifest: %w", err)
	}

	return currentImageManifest, nil
}

func getUpdatedImages(images []ContainerImage, manifest ImageManifest) ([]ContainerImage, error) {
	var updatedImages []ContainerImage
	for _, updatedImage := range images {
		for _, currentImage := range manifest.Images {
			if currentImage.Repository == updatedImage.Repository {
				updatedImage.Auth = currentImage.Auth
				updatedImage.Repository = currentImage.Repository
				updatedImage.SourceRegistry = currentImage.SourceRegistry
			}
		}

		updatedImages = append(updatedImages, updatedImage)
	}

	return updatedImages, nil
}
