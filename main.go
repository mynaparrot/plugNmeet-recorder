package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mynaparrot/plugnmeet-protocol/logging"
	"github.com/mynaparrot/plugnmeet-recorder/helpers"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/controllers"
	"github.com/mynaparrot/plugnmeet-recorder/version"
	"github.com/sirupsen/logrus"
)

func main() {
	// Define command-line flags
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to the configuration file")

	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")

	var recorderMode string
	flag.StringVar(&recorderMode, "mode", "", "Override recorder mode (recorderOnly, transcoderOnly, or both)")

	// Parse the command-line arguments
	flag.Parse()

	if showVersion {
		fmt.Printf("%s\n", version.Version)
		os.Exit(0)
	}

	// Read the application configuration
	cnf, err := helpers.ReadYamlConfigFile(configPath)
	if err != nil {
		logrus.Fatalln(err)
	}

	if recorderMode != "" {
		// Validate the input from the flag.
		switch recorderMode {
		case "both", "recorderOnly", "transcoderOnly":
			logrus.Infof("Overriding recorder mode with command-line flag: %s", recorderMode)
			cnf.Recorder.Mode = recorderMode
		default:
			logrus.Fatalf("Invalid value for -mode flag: '%s'. Allowed values are 'both', 'recorderOnly', 'transcoderOnly'.", recorderMode)
		}
	}

	// Set this config for global usage
	appCnf := config.New(cnf)

	// Setup the logger
	logger, err := logging.NewLogger(&appCnf.LogSettings)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to setup logger")
	}
	appCnf.Logger = logger

	// Prepare the server (e.g., NATS connections, JetStream)
	err = helpers.PrepareServer(appCnf)
	if err != nil {
		// Use the configured logger from this point on
		appCnf.Logger.Fatalln(err)
	}

	// Setup context for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	// cancel all context when main function exit
	defer cancel()

	// Start recorder services
	rc := controllers.NewRecorderController(ctx, appCnf, logger)
	rc.BootUp()

	// Defer closing connections to ensure they are cleaned up
	// when the main function exits
	defer helpers.HandleCloseConnections(appCnf)

	// Wait for interrupt signal to gracefully shut down the server.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	appCnf.Logger.Infoln("exit requested, shutting down services...")
	rc.CallEndToAll()
}
