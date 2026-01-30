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
	// add this recorder to the bucket
	err := c.ns.AddRecorder(pingInterval)
	if err != nil {
		c.logger.WithError(err).Fatal("failed to add this recorder to the bucket")
	}
	// now start ping
	go c.startPing()

	// try to recover if panic happens
	defer func() {
		if r := recover(); r != nil {
			c.logger.Warnln("recovered from panic in", r)
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
	}).Infof("=== recorder is ready v%s ====", version.Version)
}

func (c *RecorderController) CallEndToAll() {
	c.logger.Infoln("received request to close all recorders")
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
	c.logger.Infoln("all recorders closed")
}

func (c *RecorderController) startPing() {
	ping := time.NewTicker(pingInterval)

	for {
		select {
		case <-c.closeTicker:
			return
		case <-ping.C:
			err := c.ns.UpdateLastPing()
			if err != nil {
				c.logger.Errorln(err)
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

	err := c.ns.UpdateCurrentProgress(count)
	if err != nil {
		c.logger.WithError(err).Errorln("failed to update current progress")
	}

	return count
}
