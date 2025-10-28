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
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

const taskIDTemplate = "%d-%d"

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
		ctx:         ctx,
		cnf:         cnf,
		ns:          ns,
		notifier:    utils.NewNotifier(cnf.PlugNmeetInfo.Host, cnf.PlugNmeetInfo.ApiKey, cnf.PlugNmeetInfo.ApiSecret, nil),
		logger:      logger.WithField("component", "recorder-controller"),
		closeTicker: make(chan bool),
	}
}

func (c *RecorderController) BootUp() {
	// add this recorder to the bucket
	err := c.ns.AddRecorder()
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
		c.startRecordingService()
	case "transcoderOnly":
		c.startTranscodingService()
	default:
		// by default, it will be both
		c.startRecordingService()
		c.startTranscodingService()
	}

	c.logger.WithFields(logrus.Fields{
		"recorderId": c.cnf.Recorder.Id,
		"version":    version.Version,
		"runtime":    runtime.Version(),
		"mode":       c.cnf.Recorder.Mode,
	}).Infof("=== recorder is ready V:%s ====", version.Version)
}

func (c *RecorderController) startRecordingService() {
	// subscribe to channel for receiving tasks
	_, err := c.cnf.NatsConn.Subscribe(c.cnf.NatsInfo.Recorder.RecorderChannel, func(msg *nats.Msg) {
		req := new(plugnmeet.PlugNmeetToRecorder)
		err := proto.Unmarshal(msg.Data, req)
		if err != nil {
			c.logger.Errorln(err)
			return
		}
		if req.From != "plugnmeet" {
			return
		}

		// Create a contextual logger for each task. This is very powerful!
		taskLogger := c.logger.WithFields(logrus.Fields{
			"task":        req.Task,
			"roomTableId": req.RoomTableId,
			"sid":         req.RoomSid,
			"roomId":      req.RoomId,
		})

		switch req.Task {
		case plugnmeet.RecordingTasks_START_RECORDING,
			plugnmeet.RecordingTasks_START_RTMP:
			if req.RecorderId == c.cnf.Recorder.Id {
				res := &plugnmeet.CommonResponse{
					Status: true,
					Msg:    "success",
				}
				// Pass the contextual logger to the handler
				err := c.handleStartTask(req, taskLogger)
				if err != nil {
					res.Status = false
					res.Msg = err.Error()
				}
				marshal, _ := proto.Marshal(res)
				err = msg.Respond(marshal)
				if err != nil {
					taskLogger.Errorln(err)
				}
			}
		case plugnmeet.RecordingTasks_STOP_RECORDING,
			plugnmeet.RecordingTasks_STOP_RTMP,
			plugnmeet.RecordingTasks_STOP:
			// Pass the contextual logger to the handler
			ok := c.handleStopTask(req, taskLogger)
			if ok {
				// then the process was in this recorder
				res := &plugnmeet.CommonResponse{
					Status: true,
					Msg:    "success",
				}
				marshal, _ := proto.Marshal(res)
				err := msg.Respond(marshal)
				if err != nil {
					taskLogger.Errorln(err)
				}
			}
		default:
			taskLogger.Errorf("invalid task %s received", req.Task.String())
		}
	})

	if err != nil {
		c.logger.Fatal(err)
	}
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
	c.logger.Infoln("all recorders closed")
}

func (c *RecorderController) startPing() {
	ping := time.NewTicker(3 * time.Second)

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
