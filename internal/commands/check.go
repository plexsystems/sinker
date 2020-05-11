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
)

// NewCheckCommand creates a new list command
func NewCheckCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "check",
		Short: "Check for newer images in the remote registry",
		Args:  cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCheckCommand(args); err != nil {
				return fmt.Errorf("check: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runCheckCommand(args []string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working dir: %w", err)
	}

	sourcePath := filepath.Join(workingDir, args[0])

	sourceImages, err := GetImagesInPath(sourcePath)
	if err != nil {
		return fmt.Errorf("get before list: %w", err)
	}

	api, err := registry.New("https://registry-1.docker.io/", "", "")
	if err != nil {
		return fmt.Errorf("new registry client: %w", err)
	}
	api.Logf = registry.Quiet

	for _, sourceImage := range sourceImages {
		var newerVersions []string

		if sourceImage.Host == "quay.io" {
			fmt.Printf("Image %s has quay.io address, skipping...\n", sourceImage)
			continue
		}

		sourceTag, err := version.NewVersion(sourceImage.Version)
		if err != nil {
			fmt.Printf("skipping %v: %v\n", sourceImage, err)
			continue
		}

		var searchRepo string
		if !strings.Contains(sourceImage.Repository, "/") {
			searchRepo = "library/" + sourceImage.Repository
		} else {
			searchRepo = sourceImage.Repository
		}

		allTags, err := api.Tags(searchRepo)
		if err != nil {
			return fmt.Errorf("getting tags: %w", err)
		}

		allTags = removeLatestTags(allTags)
		for _, tag := range allTags {
			upstreamTag, err := version.NewVersion(tag)
			if err != nil {
				fmt.Printf("skipping %v: %v\n", sourceImage, err)
				continue
			}

			if sourceTag.LessThan(upstreamTag) {
				newerVersions = append(newerVersions, upstreamTag.Original())
			}
		}

		if len(newerVersions) > 0 {
			fmt.Printf("New versions for %v found: %v\n", sourceImage, newerVersions)
		} else {
			fmt.Printf("%v is up to date!\n", sourceImage)
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
