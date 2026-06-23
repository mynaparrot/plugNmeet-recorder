package app

import (
	"context"
	"fmt"
	"os"

	"github.com/mynaparrot/plugnmeet-protocol/logging"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/controllers"
	natsservice "github.com/mynaparrot/plugnmeet-recorder/pkg/services/nats"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
	"gopkg.in/yaml.v3"
)

// provideAppConfig reads the config file and initializes the AppConfig.
func provideAppConfig(configFile string, recorderMode config.AppMode) (*config.AppConfig, error) {
	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	var appCnf config.AppConfig
	if err := yaml.Unmarshal(yamlFile, &appCnf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", configFile, err)
	}

	if recorderMode != "" {
		appCnf.Recorder.Mode = recorderMode
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	appCnf.RootWorkingDir = wd

	// Initialize the configuration, setting default values and creating necessary directories.
	initializedAppCnf, err := config.Initialize(&appCnf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize config: %w", err)
	}

	return initializedAppCnf, nil
}

// provideLogger initializes the application logger and livekit protocol logger.
func provideLogger(appCnf *config.AppConfig) (*logrus.Logger, error) {
	logger, err := logging.NewLogger(&appCnf.LogSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to setup logger: %w", err)
	}

	return logger, nil
}

// ExecutePreStartTasks runs essential setup tasks that depend on core components
// like the application context, configuration, and logger.
// In the future, other pre-start logic can be added here.
func ExecutePreStartTasks(ctx context.Context, appCnf *config.AppConfig, logger *logrus.Logger) error {
	if appCnf.Hooks != nil {
		if err := appCnf.Hooks.InitializeHooks(ctx, appCnf, logger); err != nil {
			logger.WithError(err).Error("failed to initialize hooks")
			return err
		}
	}
	return nil
}

var BootstrapModule = fx.Module("bootstrap",
	fx.Provide(provideAppConfig, provideLogger),
	fx.Invoke(ExecutePreStartTasks),
)

var ServiceModule = fx.Module("services",
	fx.Provide(
		natsservice.New,
		utils.NewNotifier,
	),
)

var ApplicationModule = fx.Module("application",
	BootstrapModule,
	ConnectionModule,
	ServiceModule,
	fx.Provide(controllers.NewRecorderController),
	fx.Invoke((*controllers.RecorderController).RegisterHooks),
)
