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
		Use:     path.Base(os.Args[0]),
		Short:   "imagesync",
		Long:    "A CLI tool to sync images to another registry",
		Version: "0.5.0",
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	cmd.AddCommand(newGetCommand())
	cmd.AddCommand(newSyncCommand(logger))
	cmd.AddCommand(newCheckCommand())

	return &cmd
}
