package commands

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

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

	var sources []manifest.Source
	if path == "-" {
		standardInReader := ioutil.NopCloser(bufio.NewReader(os.Stdin))
		contents, err := ioutil.ReadAll(standardInReader)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}

		imageList := strings.Split(string(contents), " ")
		sources, err = manifest.GetSourcesFromTarget(imageList, currentManifest.Target)
		if err != nil {
			return fmt.Errorf("marshal images: %w", err)
		}
	} else {
		imageManifest, err := manifest.NewWithAutodetect(currentManifest.Target.Host, currentManifest.Target.Repository, path)
		if err != nil {
			return fmt.Errorf("get new manifest: %w", err)
		}
		sources = imageManifest.Sources
	}

	for s := range sources {
		for _, currentSource := range currentManifest.Sources {
			if currentSource.Host != sources[s].Host {
				continue
			}

			if currentSource.Repository != sources[s].Repository {
				continue
			}

			// If the target host (or repository) of the source does not match the manifest
			// target host (or repository), it has been modified by the user.
			//
			// To preserve the current settings, set the manifest host and repository values
			// to the ones present in the current manifest.
			if currentSource.Target.Host != currentManifest.Target.Host {
				sources[s].Target.Host = currentSource.Target.Host
			}
			if currentSource.Target.Repository != currentManifest.Target.Repository {
				sources[s].Target.Repository = currentSource.Target.Repository
			}

			sources[s].Auth = currentSource.Auth
		}
	}

	updatedManifest := manifest.Manifest{
		Target:  currentManifest.Target,
		Sources: sources,
	}
	if err := updatedManifest.Write(outputPath); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}
