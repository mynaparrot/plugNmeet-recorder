package controllers

import (
	"errors"
	"fmt"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/recorder"
	"github.com/sirupsen/logrus"
)

// handleStartTask now accepts a logger
func (c *RecorderController) handleStartTask(req *plugnmeet.PlugNmeetToRecorder, logger *logrus.Entry) error {
	id := fmt.Sprintf(taskIDTemplate, req.RoomTableId, req.Task)
	_, ok := c.recordersInProgress.Load(id)
	if ok {
		return errors.New("this request in progress")
	}
	logger.Infoln("received new start task")

	rc := &recorder.Recorder{
		AppCnf:               c.cnf,
		Req:                  req,
		Logger:               logger.WithField("component", "recorder"),
		OnAfterStartCallback: c.onAfterStart,
		OnAfterCloseCallback: c.onAfterClose,
	}

	r := recorder.New(c.ctx, rc)
	// add in the list, otherwise if OnAfterCloseCallback called for an error,
	// then processes won't clean properly because of missing process record
	c.recordersInProgress.Store(id, r)

	var err error
	defer func() {
		if err != nil {
			// Use the injected logger for error logging
			logger.Errorln(err)
			r.Close(plugnmeet.RecordingTasks_STOP, err)
		}
	}()

	// start the process
	if err = r.Start(); err != nil {
		return err
	}

	// update progress
	count := c.updateAndGetProgress()
	logger.Infof("%d tasks in progress", count)

	return nil
}

// onAfterStart now accepts a logger
func (c *RecorderController) onAfterStart(req *plugnmeet.PlugNmeetToRecorder, logger *logrus.Entry) {
	logger.Infoln("onAfterStart callback called")

	// notify to plugnmeet
	toSend := &plugnmeet.RecorderToPlugNmeet{
		From:        "recorder",
		Status:      true,
		Task:        req.Task,
		Msg:         "success",
		RecordingId: req.RecordingId,
		RecorderId:  req.RecorderId,
		RoomTableId: req.RoomTableId,
	}
	logger.Infof("notifying to plugnmeet: %+v", toSend)

	_, err := c.notifier.NotifyToPlugNmeet(toSend)
	if err != nil {
		logger.WithError(err).Errorln("failed to notify to plugnmeet")
	}
}
