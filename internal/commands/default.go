package commands

import (
	"context"
	"log"
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewDefaultCommand creates a new default command
func NewDefaultCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:     path.Base(os.Args[0]),
		Short:   "sinker",
		Long:    "A CLI tool to sync container images to another registry",
		Version: "0.8.1",
	}

	cmd.PersistentFlags().StringP("manifest", "m", "", "Directory where the manifest file is (defaults to current directory)")
	viper.BindPFlag("manifest", cmd.PersistentFlags().Lookup("manifest"))

	ctx := context.Background()
	logger := log.New(os.Stdout, "", log.LstdFlags)

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newPullCommand(ctx, logger))
	cmd.AddCommand(newPushCommand(ctx, logger))
	cmd.AddCommand(newCheckCommand(ctx, logger))

	return &cmd
}
