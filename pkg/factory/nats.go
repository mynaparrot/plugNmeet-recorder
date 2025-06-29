package factory

import (
	"github.com/mynaparrot/plugnmeet-protocol/auth"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

const recorderUserAuthName = "PLUGNMEET_RECORDER_AUTH"

func NewNatsConnection(appCnf *config.AppConfig) error {
	tokenHandler := func() string {
		c := &plugnmeet.PlugNmeetTokenClaims{
			UserId: appCnf.Recorder.Id,
			Name:   recorderUserAuthName,
		}
		token, err := auth.GeneratePlugNmeetJWTAccessToken(appCnf.PlugNmeetInfo.ApiKey, appCnf.PlugNmeetInfo.ApiSecret, c.UserId, time.Minute*5, c)
		if err != nil {
			// This will only be fatal on the first connection attempt.
			// On reconnect, the NATS client logs the error from the handler but doesn't exit.
			log.Fatalf("failed to generate nats auth token: %v", err)
		}
		return token
	}

	opts := []nats.Option{
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
			// This will be called if MaxReconnects is reached or if the connection is closed by the server.
			// Using log.Fatal will ensure the application exits if it can't maintain a connection.
			log.Fatal("NATS connection is permanently closed.")
		}),
	}

	info := appCnf.NatsInfo
	nc, err := nats.Connect(strings.Join(info.NatsUrls, ","), opts...)
	if err != nil {
		return err
	}
	appCnf.NatsConn = nc

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	appCnf.JetStream = js

	return nil
}
