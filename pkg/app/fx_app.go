package app

import (
	"context"

	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/controllers"
	natsservice "github.com/mynaparrot/plugnmeet-recorder/pkg/services/nats"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/utils"
	log "github.com/sirupsen/logrus"
	"go.uber.org/fx"
)

var ServiceModule = fx.Module("services",
	fx.Provide(
		natsservice.New,
		utils.NewNotifier,
	),
)

func invokeHooks(ctx context.Context, appCnf *config.AppConfig, logger *log.Logger) error {
	if appCnf.Hooks != nil {
		return appCnf.Hooks.InitializeStorageHooks(ctx, appCnf, logger)
	}
	return nil
}

var ApplicationModule = fx.Module("application",
	fx.Invoke(invokeHooks),
	ConnectionModule,
	ServiceModule,
	fx.Provide(controllers.NewRecorderController),
	fx.Invoke((*controllers.RecorderController).RegisterHooks),
)
