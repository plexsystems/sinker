package commands

import (
	"log"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewDefaultCommand creates a new default command
func NewDefaultCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:     path.Base(os.Args[0]),
		Short:   "sinker",
		Long:    "A tool to sync container images to another container registry",
		Version: "0.10.0",
	}

	cmd.PersistentFlags().StringP("manifest", "m", "", "Path where the manifest file is (defaults to .images.yaml in the current directory)")
	viper.BindPFlag("manifest", cmd.PersistentFlags().Lookup("manifest"))

	logrusLogger := logrus.New()
	logrusLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: false,
	})

	log.SetOutput(logrusLogger.Writer())

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newPullCommand(logrusLogger))
	cmd.AddCommand(newPushCommand(logrusLogger))
	cmd.AddCommand(newCheckCommand(logrusLogger))

	return &cmd
}
