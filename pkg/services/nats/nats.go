package natsservice

import (
	"context"

	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"
)

type NatsService struct {
	ctx    context.Context
	logger *logrus.Entry
	app    *config.AppConfig
	nc     *nats.Conn
	js     jetstream.JetStream
}

func New(ctx context.Context, app *config.AppConfig, nc *nats.Conn, js jetstream.JetStream, logger *logrus.Logger) *NatsService {
	return &NatsService{
		ctx:    ctx,
		app:    app,
		nc:     nc,
		js:     js,
		logger: logger.WithField("service", "nats"),
	}
}
