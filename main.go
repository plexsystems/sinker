package main

import (
	"os"

	"github.com/plexsystems/sinker/internal/commands"

	"github.com/sirupsen/logrus"
)

func main() {
	logrusLogger := logrus.New()
	logrusLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: false,
	})

	if err := commands.NewDefaultCommand(logrusLogger).Execute(); err != nil {
		os.Exit(1)
	}
}
