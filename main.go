package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/app"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/version"
	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
)

func main() {
	// Define command-line flags
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to the configuration file")

	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")

	var recorderMode config.AppMode
	flag.StringVar((*string)(&recorderMode), "mode", "", "Override recorder mode (recorderOnly, transcoderOnly, or both)")

	// Parse the command-line arguments
	flag.Parse()

	if showVersion {
		fmt.Printf("%s\n", version.Version)
		os.Exit(0)
	}

	// Read config early to determine if fx.NopLogger should be used
	isDebug, err := getDebugStatus(configPath)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to read client debug status from config")
	}

	// Prepare fx options
	fxOpts := []fx.Option{
		fx.Provide(func(lc fx.Lifecycle) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logrus.Info("Shutting down application...")
					cancel()
					return nil
				},
			})
			return ctx
		}),

		fx.Supply(configPath, recorderMode),
		app.ApplicationModule,
	}
	if !isDebug {
		fxOpts = append(fxOpts, fx.NopLogger)
	}

	a := fx.New(fxOpts...)
	a.Run() // run the app

	if err := a.Err(); err != nil {
		logrus.WithError(err).Fatal("Application failed to run")
	}
}

// getDebugStatus reads the config file to determine the Debug status.
// This is a temporary read, the full config will be provided by fx.
func getDebugStatus(file string) (bool, error) {
	yamlFile, err := os.ReadFile(file)
	if err != nil {
		return false, err
	}

	var tempConfig struct {
		RecorderInfo config.RecorderInfo `yaml:"recorder"`
	}
	if err := yaml.Unmarshal(yamlFile, &tempConfig); err != nil {
		return false, err
	}

	return tempConfig.RecorderInfo.Debug, nil
}
