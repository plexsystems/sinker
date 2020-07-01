package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hashicorp/go-version"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newCheckCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "check",
		Short: "Check for newer images in the remote registry",
		Args:  cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("mirror", cmd.Flags().Lookup("mirror")); err != nil {
				return fmt.Errorf("bind flag: %w", err)
			}

			if err := runCheckCommand(args); err != nil {
				return fmt.Errorf("check: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("mirror", "m", "", "mirror prefix")

	return &cmd
}

func runCheckCommand(args []string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working dir: %w", err)
	}

	imageListPath := filepath.Join(workingDir, args[0])

	mirrorImages, err := GetImagesFromFile(imageListPath)
	if err != nil {
		return fmt.Errorf("get images from file: %w", err)
	}

	var originalImages []ContainerImage
	for _, mirrorImage := range mirrorImages {
		originalImage := getOriginalImage(mirrorImage, viper.GetString("mirror"))
		originalImages = append(originalImages, originalImage)
	}

	api, err := registry.New("https://registry-1.docker.io/", "", "")
	if err != nil {
		return fmt.Errorf("new registry client: %w", err)
	}
	api.Logf = registry.Quiet

	for _, originalImage := range originalImages {
		var newerVersions []string

		if originalImage.Host == "quay.io" {
			fmt.Printf("Image %s has quay.io address, skipping...\n", originalImage)
			continue
		}

		sourceTag, err := version.NewVersion(originalImage.Version)
		if err != nil {
			fmt.Printf("skipping %v: %v\n", originalImage, err)
			continue
		}

		var searchRepo string
		if !strings.Contains(originalImage.Repository, "/") {
			searchRepo = "library/" + originalImage.Repository
		} else {
			searchRepo = originalImage.Repository
		}

		allTags, err := api.Tags(searchRepo)
		if err != nil {
			return fmt.Errorf("getting tags: %w", err)
		}

		allTags = removeLatestTags(allTags)
		for _, tag := range allTags {
			upstreamTag, err := version.NewVersion(tag)
			if err != nil {
				fmt.Printf("skipping %v: %v\n", originalImage, err)
				continue
			}

			if sourceTag.LessThan(upstreamTag) {
				newerVersions = append(newerVersions, upstreamTag.Original())
			}
		}

		if len(newerVersions) > 0 {
			fmt.Printf("New versions for %v found: %v\n", originalImage, newerVersions)
		} else {
			fmt.Printf("%v is up to date!\n", originalImage)
		}
	}

	return nil
}

func removeLatestTags(tags []string) []string {
	var tagsWithoutLatest []string
	for _, tag := range tags {
		if unicode.IsDigit([]rune(tag)[0]) || strings.HasPrefix(tag, "v") {
			tagsWithoutLatest = append(tagsWithoutLatest, tag)
		}
	}

	return tagsWithoutLatest
}
