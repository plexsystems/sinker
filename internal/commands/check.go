package commands

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/genuinetools/reg/registry"
	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newCheckCommand(ctx context.Context, logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "check",
		Short: "Check for newer images in the source registry",

		RunE: func(cmd *cobra.Command, args []string) error {
			manifestDirectory := viper.GetString("manifest")
			if err := runCheckCommand(ctx, logger, manifestDirectory); err != nil {
				return fmt.Errorf("check: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runCheckCommand(ctx context.Context, logger *log.Logger, directory string) error {
	dockerOpts := registry.Opt{
		Insecure: true,
		Domain:   "https://index.docker.io",
	}

	dockerRegistry, err := registry.New(ctx, types.AuthConfig{}, dockerOpts)
	if err != nil {
		return fmt.Errorf("new registry: %w", err)
	}

	manifest, err := GetManifest(viper.GetString("manifest"))
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	for _, image := range manifest.Images {
		if image.Host != "docker.io" {
			logger.Printf("Image %s not sourced from docker.io. Skipping ...", image.String())
			continue
		}

		imageVersion, err := version.NewVersion(image.Tag)
		if err != nil {
			logger.Printf("Image %s version did not parse correctly. Skipping ...", image.String())
			continue
		}

		imageTags, err := dockerRegistry.Tags(ctx, image.Repository)
		if err != nil {
			return fmt.Errorf("fetch tags: %w", err)
		}

		imageTags = filterTags(imageTags)

		newerVersions, err := getNewerVersions(imageVersion, imageTags)
		if err != nil {
			return fmt.Errorf("getting newer version: %w", err)
		}

		if len(newerVersions) == 0 {
			logger.Printf("Image %v is up to date!", image.String())
			continue
		}

		logger.Printf("New versions for %v found: %v", image.String(), newerVersions)
	}

	return nil
}

func getNewerVersions(currentVersion *version.Version, foundTags []string) ([]string, error) {
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

	if len(newerVersions) > 5 {
		newerVersions = newerVersions[len(newerVersions)-5:]
	}

	return newerVersions, nil
}

func filterTags(tags []string) []string {
	var filteredTags []string
	for _, tag := range tags {
		if strings.Contains(tag, ".") && !strings.Contains(tag, "-") {
			filteredTags = append(filteredTags, tag)
		}
	}

	return filteredTags
}
