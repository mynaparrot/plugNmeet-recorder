package controllers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/recorder"
	"github.com/sirupsen/logrus"
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
			// pass logger to postProcessRecording
			go c.postProcessRecording(req, filePath, fileName, log)
		} else {
			log.Errorf("avoiding postProcessRecording of: %s file because of 0 size", path.Join(filePath, fileName))
		}
	}
}

// postProcessRecording now accepts a logger
func (c *RecorderController) postProcessRecording(req *plugnmeet.PlugNmeetToRecorder, filePath, currentFileName string, logger *logrus.Entry) {
	log := logger.WithField("sub-method", "postProcessRecording")
	finalFileName := fmt.Sprintf("%s.mp4", req.RecordingId)

	log.WithFields(logrus.Fields{
		"filePath":        filePath,
		"currentFileName": currentFileName,
		"finalFileName":   finalFileName,
	}).Info("starting post recording ffmpeg process")

	if c.cnf.Recorder.PostMp4Convert {
		var args []string
		args = append(args, strings.Split(c.cnf.FfmpegSettings.PostRecording.PreInput, " ")...)
		args = append(args, "-i", path.Join(filePath, currentFileName))
		args = append(args, strings.Split(c.cnf.FfmpegSettings.PostRecording.PostInput, " ")...)
		args = append(args, path.Join(filePath, finalFileName))
		log.Infof("starting post recording ffmpeg process with args: %s", strings.Join(args, " "))

		_, err := exec.Command("ffmpeg", args...).CombinedOutput()
		if err != nil {
			log.WithError(err).Errorf("keeping the raw file: %s as output because of error from ffmpeg", currentFileName)
			// remove the new file
			_ = os.Remove(path.Join(filePath, finalFileName))
			// keep the old file as output
			finalFileName = currentFileName
		} else {
			err = os.Remove(path.Join(filePath, currentFileName))
			if err != nil {
				log.WithError(err).Errorln("failed to remove raw file")
			}
		}
	} else {
		// just rename
		err := os.Rename(path.Join(filePath, currentFileName), path.Join(filePath, finalFileName))
		if err != nil {
			log.Errorf("keeping the raw file: %s as output because of error during rename: %s", currentFileName, err.Error())
			// keep the old file as output
			finalFileName = currentFileName
		}
	}

	outputFilePath := path.Join(filePath, finalFileName)
	stat, err := os.Stat(outputFilePath)
	if err != nil {
		log.WithError(err).Errorln("failed to stat output file")
		return
	}

	size := float32(stat.Size()) / 1000000.0
	var relativePath string

	// To robustly calculate the relative path, first ensure both paths are absolute.
	// This prevents errors when paths are configured relatively (e.g., "./recordings").
	basePath, err := filepath.Abs(c.cnf.Recorder.CopyToPath.MainPath)
	if err != nil {
		log.WithError(err).Errorf("could not determine absolute path for main_path '%s', falling back to string trimming", c.cnf.Recorder.CopyToPath.MainPath)
		relativePath = strings.TrimPrefix(outputFilePath, c.cnf.Recorder.CopyToPath.MainPath) // fallback
	}

	absOutputFilePath, err := filepath.Abs(outputFilePath)
	if err != nil {
		log.WithError(err).Errorf("could not determine absolute path for output_file_path '%s', falling back to string trimming", outputFilePath)
		relativePath = strings.TrimPrefix(outputFilePath, c.cnf.Recorder.CopyToPath.MainPath) // fallback
	}

	relativePath, err = filepath.Rel(basePath, absOutputFilePath)
	if err != nil {
		log.WithFields(logrus.Fields{
			"base_path":   basePath,
			"output_path": absOutputFilePath,
		}).WithError(err).Warnf("could not make path relative for %s, falling back to string trimming", absOutputFilePath)
		relativePath = strings.TrimPrefix(absOutputFilePath, basePath)
	}

	toSend := &plugnmeet.RecorderToPlugNmeet{
		From:        "recorder",
		Status:      true,
		Task:        plugnmeet.RecordingTasks_RECORDING_PROCEEDED,
		Msg:         "success",
		RecordingId: req.RecordingId,
		RecorderId:  req.RecorderId,
		RoomTableId: req.RoomTableId,
		FilePath:    relativePath,
		FileSize:    float32(int(size*100)) / 100,
	}
	log.Infof("notifying to plugnmeet with data: %+v", toSend)

	_, err = c.notifier.NotifyToPlugNmeet(toSend)
	if err != nil {
		log.WithError(err).Errorln("failed to notify to plugnmeet")
	}

	// post-processing scripts
	if len(c.cnf.Recorder.PostProcessingScripts) == 0 {
		return
	}
	data := map[string]interface{}{
		"recording_id":  req.GetRecordingId(),
		"room_table_id": req.GetRoomTableId(),
		"room_id":       req.GetRoomId(),
		"room_sid":      req.GetRoomSid(),
		"file_name":     finalFileName,
		"file_path":     path.Join(filePath, finalFileName), // this will be the full path of the file
		"file_size":     size,
		"recorder_id":   req.GetRecorderId(),
	}
	marshal, err := json.Marshal(data)
	if err != nil {
		log.WithError(err).Errorln("failed to marshal post-processing data")
		return
	}

	for _, script := range c.cnf.Recorder.PostProcessingScripts {
		_, err := exec.Command("/bin/sh", script, string(marshal)).CombinedOutput()
		if err != nil {
			logger.WithError(err).Errorln("failed to run post-processing script")
		}
	}
	log.Infoln("post process recording has been finished")
}
