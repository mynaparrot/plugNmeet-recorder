package app

import (
	"context"
	"strings"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/auth"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
)

const recorderUserAuthName = "PLUGNMEET_RECORDER_AUTH"

func provideNATSConnection(lc fx.Lifecycle, appCnf *config.AppConfig, log *logrus.Logger) (*nats.Conn, error) {
	tokenHandler := func() string {
		c := &plugnmeet.PlugNmeetTokenClaims{
			UserId: appCnf.Recorder.Id,
			Name:   recorderUserAuthName,
		}
		token, err := auth.GeneratePlugNmeetJWTAccessToken(appCnf.PlugNmeetInfo.ApiKey, appCnf.PlugNmeetInfo.ApiSecret, c.UserId, time.Minute*5, c)
		if err != nil {
			// This will only be fatal on the first connection attempt.
			// On reconnect, the NATS client logs the error from the handler but doesn't exit.
			log.Fatalf("Failed to generate NATS auth token: %v", err)
		}
		return token
	}

	opts := []nats.Option{
		nats.Name("plugnmeet-recorder"),
		nats.TokenHandler(tokenHandler),
		nats.ReconnectWait(5 * time.Second),
		nats.MaxReconnects(-1), // Keep trying to reconnect indefinitely.
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Errorf("NATS disconnected with error: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Infof("NATS reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Warn("NATS connection is permanently closed.")
		}),
	}

	info := appCnf.NatsInfo
	nc, err := nats.Connect(strings.Join(info.NatsUrls, ","), opts...)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Info("Closing NATS connection")
			if err := nc.Drain(); err != nil {
				return err
			}
			nc.Close()
			return nil
		},
	})

	return nc, nil
}

func provideJetStream(nc *nats.Conn, logger *logrus.Logger) (jetstream.JetStream, error) {
	l := logger.WithField("method", "provideJetStream")
	js, err := jetstream.New(nc)
	if err != nil {
		l.WithError(err).Error("failed to create jetstream context")
		return nil, err
	}
	return js, nil
}

var ConnectionModule = fx.Module("connections",
	// Providers for each connection type
	fx.Provide(
		provideNATSConnection,
		provideJetStream,
	),
)
