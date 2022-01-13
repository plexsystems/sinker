package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "version",
		Short: "The version of sinker",

		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sinker version %v\n", sinkerVersion)
		},
	}

	return &cmd
}
