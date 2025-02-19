package factory

import (
	"github.com/mynaparrot/plugnmeet-protocol/auth"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"strings"
	"time"
)

const nameAsUser = "PLUGNMEET_RECORDER_AUTH"

func NewNatsConnection(appCnf *config.AppConfig) error {
	c := &plugnmeet.PlugNmeetTokenClaims{
		UserId: appCnf.Recorder.Id,
		Name:   nameAsUser,
	}
	token, err := auth.GeneratePlugNmeetJWTAccessToken(appCnf.PlugNmeetInfo.ApiKey, appCnf.PlugNmeetInfo.ApiSecret, c.UserId, time.Minute*1, c)
	if err != nil {
		return err
	}

	info := appCnf.NatsInfo
	nc, err := nats.Connect(strings.Join(info.NatsUrls, ","), nats.Token(token))
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
