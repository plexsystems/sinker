package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/plexsystems/sinker/internal/manifest"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newListCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:       "list <source|target>",
		Short:     "List the images found in the manifest",
		Args:      cobra.ExactValidArgs(1),
		ValidArgs: []string{"source", "target"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("output", cmd.Flags().Lookup("output")); err != nil {
				return fmt.Errorf("bind output flag: %w", err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			origin := args[0]

			if err := runListCommand(origin); err != nil {
				return fmt.Errorf("list: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Output the images in the manifest to a file")

	return &cmd
}

func runListCommand(origin string) error {
	manifestPath := viper.GetString("manifest")

	imageManifest, err := manifest.Get(manifestPath)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	var images []string
	for _, source := range imageManifest.Sources {
		if strings.EqualFold(origin, "target") {
			images = append(images, source.TargetImage())
		} else {
			images = append(images, source.Image())
		}
	}

	// When the output flag is not provided, print the images to the console.
	if viper.GetString("output") == "" {
		for _, image := range images {
			fmt.Println(image)
		}

		return nil
	}

	fileList, err := os.Create(viper.GetString("output"))
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer fileList.Close()

	for _, value := range images {
		if _, err := fmt.Fprintln(fileList, value); err != nil {
			return fmt.Errorf("writing image to file: %w", err)
		}
	}

	if err := fileList.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	return nil
}
