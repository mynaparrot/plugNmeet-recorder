package helpers

import (
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
)

func HandleCloseConnections(appCnf *config.AppConfig) {
	// close nats
	_ = appCnf.NatsConn.Drain()
	appCnf.NatsConn.Close()

	// close logger
	appCnf.Logger.Exit(0)
}
