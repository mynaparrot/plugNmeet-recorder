package controllers

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/recorder"
	natsservice "github.com/mynaparrot/plugnmeet-recorder/pkg/services/nats"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/utils"
	"github.com/mynaparrot/plugnmeet-recorder/version"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
)

const (
	taskIDTemplate = "%d-%d"
	pingInterval   = 3 * time.Second
)

type RecorderController struct {
	ctx                 context.Context
	cnf                 *config.AppConfig
	notifier            *utils.Notifier
	nc                  *nats.Conn
	js                  jetstream.JetStream
	ns                  *natsservice.NatsService
	logger              *logrus.Entry
	closeTicker         chan bool
	recordersInProgress sync.Map
	isShuttingDown      atomic.Bool
}

func NewRecorderController(ctx context.Context, cnf *config.AppConfig, nc *nats.Conn, js jetstream.JetStream, ns *natsservice.NatsService, notifier *utils.Notifier, logger *logrus.Logger) *RecorderController {
	log := logger.WithFields(logrus.Fields{
		"component":  "recorder-controller",
		"recorderId": cnf.Recorder.Id,
	})

	return &RecorderController{
		ctx:         ctx,
		cnf:         cnf,
		nc:          nc,
		js:          js,
		ns:          ns,
		notifier:    notifier,
		closeTicker: make(chan bool),
		logger:      log,
	}
}

func (c *RecorderController) RegisterHooks(lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			return c.bootUp()
		},
		OnStop: func(_ context.Context) error {
			c.callEndToAll()
			return nil
		},
	})
}

func (c *RecorderController) bootUp() error {
	// Only register as an active recorder if we are in a mode that can handle recordings.
	if c.cnf.Recorder.Mode != config.ModeTranscoderOnly {
		c.logger.Info("Registering as an active recorder")
		// add this recorder to the bucket
		if err := c.ns.RegisterAsActiveRecorder(pingInterval); err != nil {
			c.logger.WithError(err).Error("failed to add this recorder to the bucket")
			return err
		}
		// now start ping
		go c.startPing()
	}

	// try to recover if panic happens
	defer func() {
		if r := recover(); r != nil {
			c.logger.Warnln("Recovered from panic in", r)
		}
	}()

	switch c.cnf.Recorder.Mode {
	case config.ModeRecorderOnly:
		go c.startRecordingService()
	case config.ModeTranscoderOnly:
		go c.startTranscodingService()
	default:
		// by default, it will be both
		go c.startRecordingService()
		go c.startTranscodingService()
	}

	c.logger.WithFields(logrus.Fields{
		"recorderId": c.cnf.Recorder.Id,
		"version":    version.Version,
		"runtime":    runtime.Version(),
		"mode":       c.cnf.Recorder.Mode,
	}).Infof("=== Recorder is ready v%s ====", version.Version)
	return nil
}

func (c *RecorderController) callEndToAll() {
	if !c.isShuttingDown.CompareAndSwap(false, true) {
		return // already shutting down
	}

	c.logger.Infoln("Received request to shut down services")

	// Only perform recorder-specific cleanup if we are not in transcoderOnly mode.
	if c.cnf.Recorder.Mode != config.ModeTranscoderOnly {
		c.logger.Infoln("Closing all active recorders...")
		var wg sync.WaitGroup
		c.recordersInProgress.Range(func(key, value interface{}) bool {
			if process, ok := value.(*recorder.Recorder); ok {
				wg.Add(1)
				go func() {
					defer wg.Done()
					process.Close(plugnmeet.RecordingTasks_STOP, nil)
				}()
			}
			return true
		})

		wg.Wait()
		close(c.closeTicker)
		c.ns.DeleteRecorder()
		c.logger.Infoln("All recorders closed and unregistered")
	}

	c.logger.Infoln("Shutdown complete")
}

func (c *RecorderController) startPing() {
	ping := time.NewTicker(pingInterval)

	for {
		select {
		case <-c.closeTicker:
			return
		case <-ping.C:
			if c.isShuttingDown.Load() {
				return
			}
			if err := c.ns.UpdateStatus(); err != nil {
				c.logger.WithError(err).Error("Failed to update status")
			}
		}
	}
}

func (c *RecorderController) updateAndGetProgress() int {
	var count int
	c.recordersInProgress.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if err := c.ns.UpdateCurrentProgress(count); err != nil {
		c.logger.WithError(err).Errorln("Failed to update current progress")
	}

	return count
}
