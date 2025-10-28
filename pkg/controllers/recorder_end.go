package controllers

import (
	"fmt"
	"os"
	"path"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/recorder"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// handleStopTask now accepts a logger
func (c *RecorderController) handleStopTask(req *plugnmeet.PlugNmeetToRecorder, logger *logrus.Entry) bool {
	log := logger.WithFields(logrus.Fields{
		"task":        req.Task,
		"roomTableId": req.RoomTableId,
		"sid":         req.RoomSid,
		"roomId":      req.RoomId,
		"method":      "handleStopTask",
	})
	log.Infoln("received new stop task")

	var tasksToCheck []plugnmeet.RecordingTasks
	switch req.Task {
	case plugnmeet.RecordingTasks_STOP_RECORDING:
		tasksToCheck = append(tasksToCheck, plugnmeet.RecordingTasks_START_RECORDING)
	case plugnmeet.RecordingTasks_STOP_RTMP:
		tasksToCheck = append(tasksToCheck, plugnmeet.RecordingTasks_START_RTMP)
	case plugnmeet.RecordingTasks_STOP:
		// For a general STOP, try to stop both recording and RTMP.
		tasksToCheck = append(tasksToCheck, plugnmeet.RecordingTasks_START_RECORDING, plugnmeet.RecordingTasks_START_RTMP)
	}

	var found bool
	for _, task := range tasksToCheck {
		// pass logger to getAndDeleteRecorderInProgress
		if process, ok := c.getAndDeleteRecorderInProgress(req.RoomTableId, task, log); ok && process != nil {
			// need to start the process in goroutine otherwise will be delay in reply,
			// and this will show error in the client
			go process.Close(req.Task, nil)
			found = true
		}
	}

	return found
}

// getAndDeleteRecorderInProgress now accepts a logger
func (c *RecorderController) getAndDeleteRecorderInProgress(tableId int64, task plugnmeet.RecordingTasks, log *logrus.Entry) (*recorder.Recorder, bool) {
	id := fmt.Sprintf(taskIDTemplate, tableId, task)
	val, ok := c.recordersInProgress.LoadAndDelete(id)
	if !ok {
		return nil, false
	}
	process, ok := val.(*recorder.Recorder)
	if !ok {
		log.Errorf("invalid type in recordersInProgress for id %s", id)
		return nil, false
	}
	return process, true
}

// onAfterClose now accepts a logger
func (c *RecorderController) onAfterClose(req *plugnmeet.PlugNmeetToRecorder, filePath, fileName string, processErr error, logger *logrus.Entry) {
	log := logger.WithField("method", "onAfterClose").WithError(processErr)
	log.Infoln("onAfterClose callback called")

	// Atomically remove from map. This handles cleanup for crashes or other unexpected closures.
	// It's safe to call even if handleStopTask already removed it.
	c.recordersInProgress.Delete(fmt.Sprintf(taskIDTemplate, req.RoomTableId, req.Task))

	// decrement process
	err := c.ns.UpdateCurrentProgress(false)
	if err != nil {
		log.WithError(err).Errorln("failed to update current progress")
	}

	// notify to plugnmeet
	toSend := &plugnmeet.RecorderToPlugNmeet{
		From:        "recorder",
		Status:      true,
		Msg:         "success",
		Task:        plugnmeet.RecordingTasks_END_RECORDING,
		RecordingId: req.RecordingId,
		RecorderId:  req.RecorderId,
		RoomTableId: req.RoomTableId,
	}
	if req.Task == plugnmeet.RecordingTasks_START_RTMP {
		toSend.Task = plugnmeet.RecordingTasks_END_RTMP
	}
	if processErr != nil {
		toSend.Status = false
		toSend.Msg = processErr.Error()
	}
	log.Infof("notifying to plugnmeet with data: %+v", toSend)

	_, err = c.notifier.NotifyToPlugNmeet(toSend)
	if err != nil {
		log.WithError(err).Errorln("failed to notify to plugnmeet")
	}

	if req.Task == plugnmeet.RecordingTasks_START_RECORDING {
		stat, err := os.Stat(path.Join(filePath, fileName))
		if err != nil {
			switch {
			case os.IsNotExist(err) && processErr != nil:
				// in this case, not found error is expected so, don't need to log
				// otherwise will create confusion
			default:
				log.WithError(err).Errorln("failed to stat output file")
			}
			return
		}
		if stat.Size() > 0 {
			task := &plugnmeet.TranscodingTask{
				RecordingId: req.RecordingId,
				RoomId:      req.RoomId,
				RoomSid:     req.RoomSid,
				FilePath:    filePath,
				FileName:    fileName,
				RoomTableId: req.RoomTableId,
				RecorderId:  req.RecorderId,
			}

			marshal, err := proto.Marshal(task)
			if err != nil {
				log.WithError(err).Errorln("failed to marshal transcoding task")
				return
			}

			_, err = c.cnf.JetStream.Publish(c.ctx, c.cnf.NatsInfo.Recorder.TranscodingJobs, marshal)
			if err != nil {
				log.WithError(err).Errorln("failed to publish transcoding task")
			}
		} else {
			log.Errorf("avoiding to publish transcoding task of: %s file because of 0 size", path.Join(filePath, fileName))
		}
	}
}
