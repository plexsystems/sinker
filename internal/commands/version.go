package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:       "version",
		Short:     "The version of sinker",

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runVersionCommand(); err != nil {
				return fmt.Errorf("list: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Output the images in the manifest to a file")

	return &cmd
}

func runVersionCommand() error {
	fmt.Println("sinker version " + sinkerVersion)
	return nil
}
