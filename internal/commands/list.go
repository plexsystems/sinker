package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newListCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:       "list <source|target>",
		Short:     "List the images found in the image manifest",
		Args:      cobra.OnlyValidArgs,
		ValidArgs: []string{"", "source", "target"},

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("output", cmd.Flags().Lookup("output")); err != nil {
				return fmt.Errorf("bind output flag: %w", err)
			}

			var location string
			if len(args) > 0 {
				location = args[0]
			}

			if err := runListCommand(".", location); err != nil {
				return fmt.Errorf("list: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Output the images in the manifest to a file")

	return &cmd
}

func runListCommand(path string, location string) error {
	manifest, err := GetManifest()
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	var listImages []string
	for _, image := range manifest.Images {
		if location == "source" {
			listImages = append(listImages, image.SourceImage())
		} else {
			listImages = append(listImages, image.TargetImage())
		}
	}

	if viper.GetString("output") == "" {
		for _, image := range listImages {
			fmt.Println(image)
		}
		return nil
	}

	f, err := os.Create(viper.GetString("output"))
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	for _, value := range listImages {
		if _, err := fmt.Fprintln(f, value); err != nil {
			return fmt.Errorf("writing image to file: %w", err)
		}
	}

	return nil
}
