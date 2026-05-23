package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-protocol/utils"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"mvdan.cc/sh/v3/shell"
)

const maxTranscodingRetries = 3

func (c *RecorderController) startTranscodingService() {
	logger := c.logger.WithField("service", "transcoder")

	consumer, err := c.cnf.JetStream.Consumer(c.ctx, c.cnf.NatsInfo.Recorder.TranscodingJobs, utils.TranscoderConsumerDurable)
	if err != nil {
		logger.WithError(err).Fatalln("Failed to create consumer for transcoding jobs")
	}
	cnf := c.cnf.Recorder

	logger.Infoln("Transcoding service started successfully")

	// Single loop ensures that only one job is processed at a time by this worker instance.
	for {
		select {
		case <-c.ctx.Done():
			logger.Infoln("Closing transcoding worker")
			return
		default:
			// In "both" mode, we check CPU usage before fetching a new transcoding job when PostMp4Convert = true
			// If usage is high, the service pauses to prioritize active recordings.
			// This prevents transcoding from blocking live sessions and avoids NATS message retry failures.
			if cnf.Mode == config.ModeBoth && cnf.PostMp4Convert && cnf.TranscodingCpuLimitBothMode != nil && *cnf.TranscodingCpuLimitBothMode > 0 {
				percents, err := cpu.Percent(time.Second, false)
				if err != nil {
					logger.WithError(err).Errorln("failed to get cpu usage, will proceed to fetch job")
				} else if len(percents) > 0 {
					if percents[0] > *cnf.TranscodingCpuLimitBothMode {
						logger.Warnf("cpu usage %f is higher than threshold %f. delaying fetching new transcoding task", percents[0], *cnf.TranscodingCpuLimitBothMode)
						// wait before checking again, also check for context cancellation
						select {
						case <-time.After(15 * time.Second):
							continue // restart the loop
						case <-c.ctx.Done():
							logger.Infoln("context cancelled, closing transcoding worker")
							return
						}
					}
				}
			}

			// Fetch will block until a message is available or the timeout is reached.
			// Using FetchMaxWait to prevent indefinite blocking if context is canceled.
			batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
			if err != nil {
				// If there are no messages, continue and try again.
				if errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				logger.WithError(err).Errorln("Failed to fetch messages")
				// Backoff before retrying on other errors
				time.Sleep(2 * time.Second)
				continue
			}

			// Since we fetch 1, there will only be one message.
			for msg := range batch.Messages() {
				meta, err := msg.Metadata()
				if err != nil {
					logger.WithError(err).Errorln("Failed to get message metadata")
					_ = msg.NakWithDelay(5 * time.Second)
					continue
				}

				if meta.NumDelivered > maxTranscodingRetries {
					logger.Warnf("Transcoding job failed after %d attempts, removing from queue", maxTranscodingRetries)
					// Ack the message to prevent it from being redelivered
					_ = msg.Ack()
					continue
				}

				task := new(plugnmeet.TranscodingTask)
				if err := proto.Unmarshal(msg.Data(), task); err != nil {
					logger.WithError(err).Errorln("Failed to unmarshal transcoding task, sending NAK")
					// If we can't even unmarshal, it's likely a bad message. Nak it with a delay.
					_ = msg.NakWithDelay(5 * time.Second)
					return
				}

				// All the tasks are a blocking call. The loop will not continue to the next Fetch until this transcoding is finished.
				log := logger.WithFields(logrus.Fields{
					"recordingId": task.RecordingId,
					"roomId":      task.RoomId,
				})
				var procErr error
				switch v := task.TaskDetails.(type) {
				case *plugnmeet.TranscodingTask_PostRecording:
					log = log.WithField("task", "post_recording")
					procErr = c.handlePostRecordingTranscoding(task, v.PostRecording, log)
				case *plugnmeet.TranscodingTask_MergeRecordings:
					log = log.WithField("task", "merge_recordings")
					procErr = c.handleMergeRecordings(task, v.MergeRecordings, log)
				}

				if procErr != nil {
					log.WithError(procErr).Warnln("Merging failed, sending NAK to re-queue job")
					_ = msg.NakWithDelay(10 * time.Second)
				} else {
					// Everything was successful, ACK the message so it's not processed again.
					if err := msg.Ack(); err != nil {
						log.WithError(err).Errorln("Failed to send ACK for completed job")
					} else {
						log.Infoln("Merging job completed and acknowledged")
					}
				}

				// send transcoding status finish event
				toSend := &plugnmeet.RecorderToPlugNmeet{
					From:        "recorder",
					Status:      true,
					Msg:         "success",
					Task:        plugnmeet.RecordingTasks_RECORDING_TRANSCODING_FINISHED,
					RecordingId: task.RecordingId,
					RecorderId:  task.RecorderId,
					RoomTableId: task.RoomTableId,
				}
				if procErr != nil {
					toSend.Status = false
					toSend.Msg = procErr.Error()
				}
				if _, err := c.notifier.NotifyToPlugNmeet(toSend); err != nil {
					log.WithError(err).Errorln("failed to notify plugnmeet")
				}
			}
		}
	}
}

func (c *RecorderController) handlePostRecordingTranscoding(task *plugnmeet.TranscodingTask, taskDetails *plugnmeet.TranscodingTaskPostRecording, log *logrus.Entry) error {
	log = log.WithFields(logrus.Fields{
		"filePath": taskDetails.FilePath,
		"fileName": taskDetails.FileName,
		"method":   "handlePostRecordingTranscoding",
	})

	rawFilePath := path.Join(taskDetails.FilePath, taskDetails.FileName)
	finalFileName := fmt.Sprintf("%s.mp4", task.RecordingId)
	outputFile := path.Join(taskDetails.FilePath, finalFileName)

	log.WithFields(logrus.Fields{
		"rawFilePath":   rawFilePath,
		"finalFileName": finalFileName,
		"outputFile":    outputFile,
	}).Info("Starting new transcoding job")

	// Check if the raw file exists before proceeding
	if _, err := os.Stat(rawFilePath); os.IsNotExist(err) {
		log.WithError(err).Errorf("Raw file not found: %s. Cannot transcode.", rawFilePath)
		// This is a permanent error for this specific file, so we ACK it to remove from queue
		// and prevent endless re-delivery.
		return nil
	}

	if c.cnf.Recorder.PostMp4Convert {
		preArgs, err := shell.Fields(c.cnf.FfmpegSettings.PostRecording.PreInput, nil)
		if err != nil {
			log.WithError(err).Errorln("Failed to parse ffmpeg pre-input args")
			return err
		}
		postArgs, err := shell.Fields(c.cnf.FfmpegSettings.PostRecording.PostInput, nil)
		if err != nil {
			log.WithError(err).Errorln("Filed to parse ffmpeg post-input args")
			return err
		}

		args := append(preArgs, "-i", rawFilePath)
		args = append(args, postArgs...)
		args = append(args, outputFile)
		log.Infof("Starting post recording ffmpeg process with args: %s", strings.Join(args, " "))

		cmd := exec.CommandContext(c.ctx, "ffmpeg", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.WithError(err).Errorf("Transcoding (ffmpeg) failed. Keeping raw file: %s as output because of error. Output: %s", taskDetails.FileName, string(output))
			// remove the new file if it was partially created
			_ = os.Remove(outputFile)
			// keep the old file as output by setting finalFileName to raw file
			finalFileName = taskDetails.FileName
			outputFile = rawFilePath
			return err // NAK will handle re-queueing
		}

		log.Infoln("Transcoding (ffmpeg) completed successfully")
		// Remove the raw file only if transcoding was successful and a new file was created
		if err = os.Remove(rawFilePath); err != nil {
			log.WithError(err).Warnf("Failed to remove raw file: %s", rawFilePath)
		}
	} else {
		// If PostMp4Convert is false, just rename the raw file to the final .mp4 name
		// This assumes the raw file is already in a playable format or intended to be kept as is.
		if err := os.Rename(rawFilePath, outputFile); err != nil {
			log.WithError(err).Errorf("Keeping the raw file: %s as output because of error during rename: %s", taskDetails.FileName, err.Error())
			// keep the old file as output
			finalFileName = taskDetails.FileName
			outputFile = rawFilePath
			return err // NAK will handle re-queueing
		}
		log.Infoln("Raw file renamed to final output file")
	}

	// Calculate file size and relative path for notification
	stat, err := os.Stat(outputFile)
	if err != nil {
		log.WithError(err).Errorln("Failed to stat final output file after processing")
		return err // NAK will handle re-queueing
	}

	size := float32(stat.Size()) / 1000000.0
	var relativePath string

	basePath, err := filepath.Abs(c.cnf.Recorder.CopyToPath.MainPath)
	if err != nil {
		log.WithError(err).Errorf("Could not determine absolute path for main_path '%s', falling back to string trimming", c.cnf.Recorder.CopyToPath.MainPath)
		relativePath = strings.TrimPrefix(outputFile, c.cnf.Recorder.CopyToPath.MainPath) // fallback
	} else {
		absOutputFilePath, err := filepath.Abs(outputFile)
		if err != nil {
			log.WithError(err).Errorf("Could not determine absolute path for output_file_path '%s', falling back to string trimming", outputFile)
			relativePath = strings.TrimPrefix(outputFile, c.cnf.Recorder.CopyToPath.MainPath) // fallback
		} else {
			relativePath, err = filepath.Rel(basePath, absOutputFilePath)
			if err != nil {
				log.WithFields(logrus.Fields{
					"base_path":   basePath,
					"output_path": absOutputFilePath,
				}).WithError(err).Warnf("Could not make path relative for %s, falling back to string trimming", absOutputFilePath)
				relativePath = strings.TrimPrefix(absOutputFilePath, basePath)
			}
		}
	}

	// Notify plugnmeet server about the success
	toSend := &plugnmeet.RecorderToPlugNmeet{
		From:             "recorder",
		Status:           true,
		Task:             plugnmeet.RecordingTasks_RECORDING_PROCEEDED,
		Msg:              "success",
		RecordingId:      task.RecordingId,
		RecorderId:       task.RecorderId,
		RoomTableId:      task.RoomTableId,
		FilePath:         relativePath,
		FileSize:         float32(math.Round(float64(size)*100) / 100),
		RecordingVariant: &taskDetails.RecordingVariant,
	}

	c.notifyAndRunPostProcessingScripts(toSend, outputFile, finalFileName, size, log)
	log.Infoln("Post process recording has been finished")

	return nil
}

func (c *RecorderController) handleMergeRecordings(task *plugnmeet.TranscodingTask, taskDetails *plugnmeet.TranscodingTaskMergeRecordings, log *logrus.Entry) error {
	if len(taskDetails.FilePaths) == 0 {
		log.Errorln("no file paths provided for merging")
		return errors.New("no file paths provided for merging")
	}

	// All files should be in the same directory. We'll use the path from the first file.
	relativeOutputDir := filepath.Dir(taskDetails.FilePaths[0])
	fullOutputDir := path.Join(c.cnf.Recorder.CopyToPath.MainPath, relativeOutputDir)

	// Ensure the output directory exists
	if err := os.MkdirAll(fullOutputDir, 0755); err != nil {
		log.WithError(err).Errorf("failed to create output directory: %s", fullOutputDir)
		return err
	}

	finalFileName := fmt.Sprintf("%s.mp4", task.RecordingId)
	outputFile := path.Join(fullOutputDir, finalFileName)
	log.Infof("merging files into: %s", outputFile)

	// Create the file list for ffmpeg concat
	fileListPath := path.Join(os.TempDir(), fmt.Sprintf("ffmpeg-concat-%s.txt", task.RecordingId))
	file, err := os.Create(fileListPath)
	if err != nil {
		log.WithError(err).Errorln("failed to create ffmpeg file list")
		return err
	}
	defer os.Remove(fileListPath) // Clean up the temp file

	for _, p := range taskDetails.FilePaths {
		abs, err := filepath.Abs(path.Join(c.cnf.Recorder.CopyToPath.MainPath, p))
		if err != nil {
			file.Close()
			return err
		}

		// Check if file exists before adding to list
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			file.Close()
			log.WithError(err).Errorf("file not found for merging: %s", abs)
			return err
		}

		// Write to the concat file for ffmpeg
		if _, err := file.WriteString(fmt.Sprintf("file '%s'\n", abs)); err != nil {
			log.WithError(err).Errorln("failed to write to ffmpeg file list")
			file.Close()
			return err
		}
	}
	file.Close() // Close the file so ffmpeg can read it

	// Execute FFMPEG command: ffmpeg -f concat -safe 0 -i file_list.txt -c copy output.mp4
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", fileListPath,
		"-c", "copy",
		outputFile,
	}
	log.Infof("Starting ffmpeg merge process with args: %s", strings.Join(args, " "))

	cmd := exec.CommandContext(c.ctx, "ffmpeg", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.WithError(err).Errorf("ffmpeg merge failed. Output: %s", string(output))
		_ = os.Remove(outputFile) // remove partially created file
		return err
	}

	log.Infoln("ffmpeg merge completed successfully")

	// Notify plugnmeet
	stat, err := os.Stat(outputFile)
	if err != nil {
		log.WithError(err).Errorln("failed to stat final output file after merging")
		return err
	}

	size := float32(stat.Size()) / 1000000.0
	relativePath := path.Join(relativeOutputDir, finalFileName)

	toSend := &plugnmeet.RecorderToPlugNmeet{
		From:        "recorder",
		Status:      true,
		Task:        plugnmeet.RecordingTasks_RECORDING_PROCEEDED, // Using existing task type
		Msg:         "success",
		RecordingId: task.RecordingId,
		RecorderId:  task.RecorderId,
		RoomTableId: task.RoomTableId,
		FilePath:    relativePath,
		FileSize:    float32(math.Round(float64(size)*100) / 100),
	}
	c.notifyAndRunPostProcessingScripts(toSend, outputFile, finalFileName, size, log)

	log.Infoln("merge recordings process has been finished")
	return nil
}

func (c *RecorderController) notifyAndRunPostProcessingScripts(toSend *plugnmeet.RecorderToPlugNmeet, outputFile, finalFileName string, size float32, log *logrus.Entry) {
	log.Infof("notifying plugnmeet with data: %+v", toSend)

	if _, err := c.notifier.NotifyToPlugNmeet(toSend); err != nil {
		log.WithError(err).Errorln("failed to notify plugnmeet")
	}

	// Execute post-processing scripts
	if len(c.cnf.Recorder.PostProcessingScripts) > 0 {
		data := map[string]interface{}{
			"recording_id":  toSend.GetRecordingId(),
			"room_table_id": toSend.GetRoomTableId(),
			"room_id":       toSend.GetRoomId(),
			"room_sid":      toSend.GetRoomSid(),
			"file_name":     finalFileName,
			"file_path":     outputFile,
			"file_size":     size,
			"recorder_id":   toSend.GetRecorderId(),
		}
		marshal, err := json.Marshal(data)
		if err != nil {
			log.WithError(err).Errorln("failed to marshal post-processing data for scripts")
		} else {
			for _, script := range c.cnf.Recorder.PostProcessingScripts {
				log.Infof("running post-processing script: %s", script)
				cmd := exec.Command("/bin/sh", script, string(marshal))
				scriptOutput, scriptErr := cmd.CombinedOutput()
				if scriptErr != nil {
					log.WithError(scriptErr).Errorf("post-processing script failed: %s, Output: %s", script, string(scriptOutput))
				} else {
					log.Infof("post-processing script %s finished. Output: %s", script, string(scriptOutput))
				}
			}
		}
	}
}
