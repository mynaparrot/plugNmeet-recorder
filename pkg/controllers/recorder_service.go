package controllers

import (
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

func (c *RecorderController) startRecordingService() {
	logger := c.logger.WithField("service", "recorder")

	// subscribe to channel for receiving tasks
	_, err := c.cnf.NatsConn.Subscribe(c.cnf.NatsInfo.Recorder.RecorderChannel, func(msg *nats.Msg) {
		req := new(plugnmeet.PlugNmeetToRecorder)
		err := proto.Unmarshal(msg.Data, req)
		if err != nil {
			logger.Errorln(err)
			return
		}
		if req.From != "plugnmeet" {
			return
		}

		// Create a contextual logger for each task. This is very powerful!
		taskLogger := logger.WithFields(logrus.Fields{
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
		logger.WithError(err).Fatalln("failed to subscribe to recorder channel")
	}
	logger.Infoln("recorder service started successfully")
}
