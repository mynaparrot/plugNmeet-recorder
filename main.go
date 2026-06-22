package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/mynaparrot/plugnmeet-protocol/logging"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/app"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/version"
	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
	"gopkg.in/yaml.v3"
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

	// Read the application configuration
	cnf, err := readYamlConfigFile(configPath)
	if err != nil {
		logrus.Fatalln(err)
	}

	if recorderMode != "" {
		cnf.Recorder.Mode = recorderMode
	}

	// Set this config for global usage
	appCnf := config.Initialize(cnf)

	// Setup the logger
	logger, err := logging.NewLogger(&appCnf.LogSettings)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to setup logger")
	}

	// Prepare fx options
	fxOpts := []fx.Option{
		fx.Provide(func(lc fx.Lifecycle) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					logger.Info("Stopping application...")
					cancel()
					return nil
				},
			})
			return ctx
		}),

		fx.Supply(appCnf, logger),
		app.ApplicationModule,
	}
	if !appCnf.Recorder.Debug {
		fxOpts = append(fxOpts, fx.NopLogger)
	}

	// Run the fx application
	fx.New(fxOpts...).Run()
}

func readYamlConfigFile(file string) (*config.AppConfig, error) {
	yamlFile, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	appCnf := new(config.AppConfig)
	if err := yaml.Unmarshal(yamlFile, appCnf); err != nil {
		return nil, err
	}

	// get current working dir
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// set the root path
	appCnf.RootWorkingDir = wd

	return appCnf, err
}
