package commands

import (
	"log"
	"os"
	"path"

	"github.com/spf13/cobra"
)

// NewDefaultCommand creates a new default command
func NewDefaultCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   path.Base(os.Args[0]),
		Short: "imagesync",
		Long:  "A cli tool to sync docker images in a repository",
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewSyncCommand(logger))
	cmd.AddCommand(NewCheckCommand())

	return &cmd
}
