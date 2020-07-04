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
)

func newCheckCommand(ctx context.Context, logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "check",
		Short: "Check for newer images in the source registry",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCheckCommand(ctx, logger, "."); err != nil {
				return fmt.Errorf("check: %w", err)
			}

			return nil
		},
	}

	return &cmd
}

func runCheckCommand(ctx context.Context, logger *log.Logger, path string) error {
	dockerOpts := registry.Opt{
		Insecure: true,
		Domain:   "https://index.docker.io",
	}

	dockerRegistry, err := registry.New(ctx, types.AuthConfig{}, dockerOpts)
	if err != nil {
		return fmt.Errorf("new registry: %w", err)
	}

	manifest, err := getManifest()
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	for _, image := range manifest.Images {
		if image.SourceRegistry != "docker.io" {
			logger.Printf("Image %s not sourced from docker.io. Skipping ...", image.Source())
			continue
		}

		imageVersion, err := version.NewVersion(image.Version)
		if err != nil {
			logger.Printf("Image %s version did not parse correctly. Skipping ...", image.Source())
			continue
		}

		if !strings.Contains(image.Repository, "/") {
			image.Repository = "library/" + image.Repository
		}

		imageTags, err := dockerRegistry.Tags(ctx, image.Repository)
		if err != nil {
			return fmt.Errorf("fetch tags: %w", err)
		}
		imageTags = filterTags(imageTags)

		var newerVersions []string
		for _, tag := range imageTags {
			upstreamTag, err := version.NewVersion(tag)
			if err != nil {
				logger.Printf("Tag %v was malformed. Skipping ...", upstreamTag)
				continue
			}

			if imageVersion.LessThan(upstreamTag) {
				newerVersions = append(newerVersions, upstreamTag.Original())
			}
		}

		if len(newerVersions) == 0 {
			logger.Printf("Image %v is up to date!", image.Source())
			continue
		}

		if len(newerVersions) > 0 {
			if len(newerVersions) > 5 {
				newerVersions = newerVersions[len(newerVersions)-5:]
			}

			logger.Printf("New versions for %v found: %v", image.Source(), newerVersions)
		}
	}

	return nil
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
