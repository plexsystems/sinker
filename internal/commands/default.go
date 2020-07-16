package commands

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewDefaultCommand creates the default command.
func NewDefaultCommand(logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:     path.Base(os.Args[0]),
		Short:   "sinker",
		Long:    "A tool to sync container images to another container registry",
		Version: "0.10.0",
	}

	cmd.PersistentFlags().StringP("manifest", "m", "", "Path where the manifest file is (defaults to .images.yaml in the current directory)")
	viper.BindPFlag("manifest", cmd.PersistentFlags().Lookup("manifest"))

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newPullCommand(logger))
	cmd.AddCommand(newPushCommand(logger))
	cmd.AddCommand(newCheckCommand(logger))

	return &cmd
}
