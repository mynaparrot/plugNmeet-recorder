package natsservice

import (
	"context"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsService struct {
	ctx context.Context
	app *config.AppConfig
	nc  *nats.Conn
	js  jetstream.JetStream
}

func New(app *config.AppConfig) *NatsService {
	if app == nil {
		app = config.GetConfig()
	}
	return &NatsService{
		ctx: context.Background(),
		app: app,
		nc:  app.NatsConn,
		js:  app.JetStream,
	}
}
