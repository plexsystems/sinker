package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/plexsystems/sinker/internal/docker"
	"github.com/plexsystems/sinker/internal/manifest"

	"github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newCheckCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "check",
		Short: "Check for newer images",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("images", cmd.Flags().Lookup("images")); err != nil {
				return fmt.Errorf("bind images flag: %w", err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var input string
			if len(args) > 0 {
				input = args[0]
			}

			if err := runCheckCommand(input); err != nil {
				return fmt.Errorf("check: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringSliceP("images", "i", []string{}, "List of images to check (e.g. host.com/repo:v1.0.0)")

	return &cmd
}

func runCheckCommand(input string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := docker.New(log.Infof)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	var imagesToCheck []string
	if input == "-" {
		imagesToCheck, err = manifest.GetImagesFromStandardInput()
	} else if len(viper.GetStringSlice("images")) > 0 {
		imagesToCheck = viper.GetStringSlice("images")
	} else {
		imageManifest, err := manifest.Get(viper.GetString("manifest"))
		if err != nil {
			return fmt.Errorf("get manifest: %w", err)
		}

		for _, source := range imageManifest.Sources {
			imagesToCheck = append(imagesToCheck, source.Image())
		}
	}
	if err != nil {
		return fmt.Errorf("get images to check: %w", err)
	}

	var images []docker.RegistryPath
	for _, image := range imagesToCheck {
		images = append(images, docker.RegistryPath(image))
	}

	for _, image := range images {
		if image.Tag() == "" {
			continue
		}

		imageVersion, err := version.NewVersion(image.Tag())
		if err != nil {
			log.Infof("Image %s has an invalid version. Skipping ...", image)
			continue
		}

		tags, err := client.GetTagsForRepository(ctx, image.Host(), image.Repository())
		if err != nil {
			return fmt.Errorf("get tags: %w", err)
		}
		tags = filterTags(tags)

		newerVersions := getNewerVersions(imageVersion, tags)
		if len(newerVersions) == 0 {
			log.Infof("Image %s is up to date!", image)
			continue
		}

		log.Infof("New versions for %v found: %v", image, newerVersions)
	}

	return nil
}

func getNewerVersions(currentVersion *version.Version, foundTags []string) []string {
	var newerVersions []string
	for _, foundTag := range foundTags {
		tag, err := version.NewVersion(foundTag)
		if err != nil {
			continue
		}

		if currentVersion.LessThan(tag) {
			newerVersions = append(newerVersions, tag.Original())
		}
	}

	// For images that are very out of date, the number of newer versions can be quite long.
	// Only return the latest 5 releases to keep the list manageable.
	if len(newerVersions) > 5 {
		newerVersions = newerVersions[len(newerVersions)-5:]
		newerVersions = append([]string{"..."}, newerVersions...)
	}

	return newerVersions
}

func filterTags(tags []string) []string {
	var filteredTags []string
	for _, tag := range tags {
		semverTag, err := version.NewSemver(tag)
		if err != nil {
			continue
		}

		if !strings.EqualFold(semverTag.String(), tag) && !strings.EqualFold("v"+semverTag.String(), tag) {
			continue
		}

		// Remove tags that include architectures and other strings
		// not necessarily related to a release.
		allowedPreReleases := []string{"alpha", "beta", "rc"}
		if strings.Contains(tag, "-") && !containsSubstring(allowedPreReleases, tag) {
			continue
		}

		filteredTags = append(filteredTags, tag)
	}

	return filteredTags
}

func containsSubstring(items []string, item string) bool {
	for _, currentItem := range items {
		if strings.Contains(item, currentItem) {
			return true
		}
	}

	return false
}
