package commands

import (
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewDefaultCommand creates the default command.
func NewDefaultCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:     path.Base(os.Args[0]),
		Short:   "sinker",
		Long:    "A tool to sync container images to another container registry",
		Version: "0.10.1",
	}

	cmd.PersistentFlags().StringP("manifest", "m", "", "Path where the manifest file is (defaults to .images.yaml in the current directory)")
	viper.BindPFlag("manifest", cmd.PersistentFlags().Lookup("manifest"))

	viper.SetEnvPrefix("SINKER")
	viper.AutomaticEnv()

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newPullCommand())
	cmd.AddCommand(newPushCommand())
	cmd.AddCommand(newCheckCommand())

	return &cmd
}
