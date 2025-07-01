package controllers

import (
	"fmt"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/recorder"
	natsservice "github.com/mynaparrot/plugnmeet-recorder/pkg/services/nats"
	"github.com/mynaparrot/plugnmeet-recorder/version"
	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"runtime"
	"sync"
	"time"
)

type RecorderController struct {
	cnf                 *config.AppConfig
	ns                  *natsservice.NatsService
	closeTicker         chan bool
	recordersInProgress sync.Map
}

func NewRecorderController() *RecorderController {
	cnf := config.GetConfig()
	ns := natsservice.New(cnf)

	return &RecorderController{
		cnf:         cnf,
		ns:          ns,
		closeTicker: make(chan bool),
	}
}

func (c *RecorderController) BootUp() {
	// add this recorder to the bucket
	err := c.ns.AddRecorder()
	if err != nil {
		log.Fatal(err)
	}
	// now start ping
	go c.startPing()

	// try to recover if panic happens
	defer func() {
		if r := recover(); r != nil {
			log.Warnln("recovered from panic in", r)
		}
	}()

	// subscribe to channel for receiving tasks
	_, err = c.cnf.NatsConn.Subscribe(c.cnf.NatsInfo.Recorder.RecorderChannel, func(msg *nats.Msg) {
		req := new(plugnmeet.PlugNmeetToRecorder)
		err := proto.Unmarshal(msg.Data, req)
		if err != nil {
			log.Errorln(err)
			return
		}
		if req.From != "plugnmeet" {
			return
		}

		switch req.Task {
		case plugnmeet.RecordingTasks_START_RECORDING,
			plugnmeet.RecordingTasks_START_RTMP:
			if req.RecorderId == c.cnf.Recorder.Id {
				res := &plugnmeet.CommonResponse{
					Status: true,
					Msg:    "success",
				}
				err := c.handleStartTask(req)
				if err != nil {
					res.Status = false
					res.Msg = err.Error()
				}
				marshal, _ := proto.Marshal(res)
				err = msg.Respond(marshal)
				if err != nil {
					log.Errorln(err)
				}
			}
		case plugnmeet.RecordingTasks_STOP_RECORDING,
			plugnmeet.RecordingTasks_STOP_RTMP,
			plugnmeet.RecordingTasks_STOP:
			ok := c.handleStopTask(req)
			if ok {
				// then the process was in this recorder
				res := &plugnmeet.CommonResponse{
					Status: true,
					Msg:    "success",
				}
				marshal, _ := proto.Marshal(res)
				err := msg.Respond(marshal)
				if err != nil {
					log.Errorln(err)
				}
			}
		default:
			log.Errorln(fmt.Sprintf("invalid task %s received", req.Task.String()))
		}
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(fmt.Sprintf("recorder is ready to accept tasks, recorderId: %s; version: %s; runtime: %s", c.cnf.Recorder.Id, version.Version, runtime.Version()))
}

func (c *RecorderController) CallEndToAll() {
	c.recordersInProgress.Range(func(key, value interface{}) bool {
		if process, ok := value.(*recorder.Recorder); ok {
			process.Close(nil)
		}
		return true
	})
	close(c.closeTicker)
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
				log.Errorln(err)
			}
		}
	}
}
