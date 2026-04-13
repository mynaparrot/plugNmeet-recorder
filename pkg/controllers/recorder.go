package controllers

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/recorder"
	natsservice "github.com/mynaparrot/plugnmeet-recorder/pkg/services/nats"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/utils"
	"github.com/mynaparrot/plugnmeet-recorder/version"
	"github.com/sirupsen/logrus"
)

const (
	taskIDTemplate = "%d-%d"
	pingInterval   = 3 * time.Second
)

type RecorderController struct {
	ctx                 context.Context
	cnf                 *config.AppConfig
	notifier            *utils.Notifier
	ns                  *natsservice.NatsService
	logger              *logrus.Entry
	closeTicker         chan bool
	recordersInProgress sync.Map
}

func NewRecorderController(ctx context.Context, cnf *config.AppConfig, logger *logrus.Logger) *RecorderController {
	ns := natsservice.New(ctx, cnf)

	return &RecorderController{
		ctx:      ctx,
		cnf:      cnf,
		ns:       ns,
		notifier: utils.NewNotifier(cnf.PlugNmeetInfo.Host, cnf.PlugNmeetInfo.ApiKey, cnf.PlugNmeetInfo.ApiSecret, nil),
		logger: logger.WithFields(logrus.Fields{
			"component":  "recorder-controller",
			"recorderId": cnf.Recorder.Id,
		}),
		closeTicker: make(chan bool),
	}
}

func (c *RecorderController) BootUp() {
	// Only register as an active recorder if we are in a mode that can handle recordings.
	if c.cnf.Recorder.Mode != "transcoderOnly" {
		c.logger.Info("Registering as an active recorder")
		// add this recorder to the bucket
		if err := c.ns.RegisterAsActiveRecorder(pingInterval); err != nil {
			c.logger.WithError(err).Fatal("Failed to add this recorder to the bucket")
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
	case "recorderOnly":
		go c.startRecordingService()
	case "transcoderOnly":
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
}

func (c *RecorderController) CallEndToAll() {
	c.logger.Infoln("Received request to shut down services")

	// Only perform recorder-specific cleanup if we are not in transcoderOnly mode.
	if c.cnf.Recorder.Mode != "transcoderOnly" {
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
			if c.cnf.IsShuttingDown.Load() {
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
