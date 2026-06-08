package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	log.Infoln("Received new stop task")

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
		log.Errorf("Invalid type in recordersInProgress for id %s", id)
		return nil, false
	}
	return process, true
}

// onAfterClose now accepts a logger
func (c *RecorderController) onAfterClose(req *plugnmeet.PlugNmeetToRecorder, recordingFilePath, finalRawFilePath string, processErr error, logger *logrus.Entry) {
	log := logger.WithField("method", "onAfterClose").WithError(processErr)
	log.Infoln("onAfterClose callback called")

	// Atomically remove from map. This handles cleanup for crashes or other unexpected closures.
	// It's safe to call even if handleStopTask already removed it.
	c.recordersInProgress.Delete(fmt.Sprintf(taskIDTemplate, req.RoomTableId, req.Task))

	// update progress
	count := c.updateAndGetProgress()
	log.Infof("%d tasks left in progress", count)

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
	log.Infof("Notifying to plugnmeet with data: %+v", toSend)

	_, err := c.notifier.NotifyToPlugNmeet(toSend)
	if err != nil {
		log.WithError(err).Errorln("Failed to notify to plugnmeet")
	}

	if req.Task == plugnmeet.RecordingTasks_START_RECORDING {
		// if we used a temporary file, we must move it to the final destination first.
		if recordingFilePath != finalRawFilePath {
			log.Infof("Moving temp file %s to final destination %s", recordingFilePath, finalRawFilePath)
			err = moveFile(recordingFilePath, finalRawFilePath, log)
			if err != nil {
				log.WithError(err).Errorln("Failed to move file from temporary location")
				// if we can't move the file, we can't transcode it.
				return
			}
		}

		// now we can check the final file
		finalFilePath, finalFileName := path.Split(finalRawFilePath)
		stat, err := os.Stat(finalRawFilePath)
		if err != nil {
			switch {
			case os.IsNotExist(err) && processErr != nil:
				// in this case, not found error is expected so, don't need to log
				// otherwise will create confusion
			default:
				log.WithError(err).Errorln("Failed to stat output file")
			}
			return
		}
		if stat.Size() > 0 {
			postRecording := &plugnmeet.TranscodingTaskPostRecording{
				FileName: finalFileName, // this is the raw file name
				FilePath: finalFilePath, // this is now the final path
			}

			// Run pre-transcode scripts
			if len(c.cnf.Recorder.PreTranscodeScripts) > 0 {
				modifiedPostRecording, err := c.runPreTranscodeScripts(req, postRecording, log)
				if err != nil {
					log.WithError(err).Errorln("Pre-transcode script execution failed")
					return
				}
				postRecording = modifiedPostRecording
			}

			task := &plugnmeet.TranscodingTask{
				RecordingId: req.RecordingId,
				RoomId:      req.RoomId,
				RoomSid:     req.RoomSid,
				RoomTableId: req.RoomTableId,
				RecorderId:  req.RecorderId,
				TaskDetails: &plugnmeet.TranscodingTask_PostRecording{
					PostRecording: postRecording,
				},
			}

			marshal, err := proto.Marshal(task)
			if err != nil {
				log.WithError(err).Errorln("Failed to marshal transcoding task")
				return
			}

			if _, err = c.cnf.JetStream.Publish(c.ctx, c.cnf.NatsInfo.Recorder.TranscodingJobs, marshal); err != nil {
				log.WithError(err).Errorln("Failed to publish transcoding task")
			}
		} else {
			log.Errorf("Avoiding to publish transcoding task of: %s file because of 0 size", finalRawFilePath)
		}
	}
}

func (c *RecorderController) runPreTranscodeScripts(req *plugnmeet.PlugNmeetToRecorder, postRecording *plugnmeet.TranscodingTaskPostRecording, log *logrus.Entry) (*plugnmeet.TranscodingTaskPostRecording, error) {
	data := map[string]interface{}{
		"recording_id":  req.GetRecordingId(),
		"room_table_id": req.GetRoomTableId(),
		"room_id":       req.GetRoomId(),
		"room_sid":      req.GetRoomSid(),
		"file_name":     postRecording.FileName,
		"file_path":     postRecording.FilePath,
		"recorder_id":   req.GetRecorderId(),
	}

	var err error
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initial data for pre-transcode script: %w", err)
	}

	for _, script := range c.cnf.Recorder.PreTranscodeScripts {
		log.Infof("Running pre-transcode script: %s", script)

		// Execute the script directly, allowing the OS to respect the shebang.
		// The script's lifecycle is tied to the main application context.
		cmd := exec.CommandContext(c.ctx, script)
		cmd.Stdin = bytes.NewReader(jsonData)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("pre-transcode script %s failed: %w, stderr: %s", script, err, stderr.String())
		}

		// The output of the script becomes the input for the next one
		jsonData = out.Bytes()
	}

	// After all scripts, unmarshal the final JSON back into our struct
	var finalData map[string]interface{}
	if err := json.Unmarshal(jsonData, &finalData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal final JSON from pre-transcode scripts: %w", err)
	}

	// Update the postRecording struct with the modified values
	if filePath, ok := finalData["file_path"].(string); ok {
		postRecording.FilePath = filePath
	}
	if fileName, ok := finalData["file_name"].(string); ok {
		postRecording.FileName = fileName
	}

	return postRecording, nil
}

// moveFile moves a file from src to dst. It creates the destination directory if it doesn't exist.
func moveFile(src, dst string, log *logrus.Entry) error {
	// Ensure the destination directory exists.
	dstDir := path.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// Attempt to rename first, as it's atomic and fast.
	err := os.Rename(src, dst)
	if err == nil {
		log.Infof("Successfully moved file using rename from %s to %s", src, dst)
		return nil
	}

	// If rename fails (e.g., across different filesystems), fall back to copy-then-delete.
	log.Warnf("Rename failed (possibly cross-device move), falling back to rsync: %v", err)

	// -a: archive mode (preserves permissions, etc.)
	// --partial: keep partially transferred files for resuming.
	// --remove-source-files: move the file instead of copying.
	// --mkpath: creates the destination directory path.
	cmd := exec.Command("rsync", "-a", "--partial", "--remove-source-files", "--mkpath", src, dst)

	log.Infof("Executing rsync command: %s", cmd.String())
	output, rsyncErr := cmd.CombinedOutput()
	if rsyncErr != nil {
		return fmt.Errorf("rsync failed with error: %w. Output: %s", rsyncErr, string(output))
	}

	log.Infof("Successfully moved file using rsync. Output: %s", string(output))
	return nil
}
