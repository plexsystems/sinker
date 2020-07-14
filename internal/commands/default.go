package commands

import (
	"context"
	"log"
	"os"
	"path"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// CommandTimeout is how long to wait before timing out on a command
const CommandTimeout = 10 * time.Minute

// NewDefaultCommand creates a new default command
func NewDefaultCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:     path.Base(os.Args[0]),
		Short:   "sinker",
		Long:    "A CLI tool to sync container images to another registry",
		Version: "0.9.0",
	}

	cmd.PersistentFlags().StringP("manifest", "m", "", "Path where the manifest file is (defaults to .images.yaml in the current directory)")
	viper.BindPFlag("manifest", cmd.PersistentFlags().Lookup("manifest"))

	ctx := context.Background()

	logrusLogger := logrus.New()
	logrusLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: false,
	})

	log.SetOutput(logrusLogger.Writer())

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newPullCommand(ctx, logrusLogger))
	cmd.AddCommand(newPushCommand(ctx, logrusLogger))
	cmd.AddCommand(newCheckCommand(ctx, logrusLogger))

	return &cmd
}
