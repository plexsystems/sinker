package commands

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hashicorp/go-version"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/spf13/cobra"
)

func newCheckCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "check",
		Short: "Check for newer images in the remote registry",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCheckCommand("."); err != nil {
				return fmt.Errorf("run check: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runCheckCommand(path string) error {
	imageManifest, err := getManifest(path)
	if err != nil {
		return fmt.Errorf("get images from file: %w", err)
	}

	api, err := registry.New("https://registry-1.docker.io/", "", "")
	if err != nil {
		return fmt.Errorf("new registry client: %w", err)
	}
	api.Logf = registry.Quiet

	for _, image := range imageManifest.Images {
		var newerVersions []string

		if image.OriginRegistry == "quay.io" {
			fmt.Printf("Image %s has quay.io address, skipping...\n", image)
			continue
		}

		sourceTag, err := version.NewVersion(image.Version)
		if err != nil {
			fmt.Printf("skipping %v: %v\n", image, err)
			continue
		}

		var searchRepo string
		if !strings.Contains(image.Repository, "/") {
			searchRepo = "library/" + image.Repository
		} else {
			searchRepo = image.Repository
		}

		allTags, err := api.Tags(searchRepo)
		if err != nil {
			return fmt.Errorf("getting tags: %w", err)
		}

		allTags = removeLatestTags(allTags)
		for _, tag := range allTags {
			upstreamTag, err := version.NewVersion(tag)
			if err != nil {
				fmt.Printf("skipping %v: %v\n", image, err)
				continue
			}

			if sourceTag.LessThan(upstreamTag) {
				newerVersions = append(newerVersions, upstreamTag.Original())
			}
		}

		if len(newerVersions) > 0 {
			fmt.Printf("New versions for %v found: %v\n", image, newerVersions)
		} else {
			fmt.Printf("%v is up to date!\n", image)
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
