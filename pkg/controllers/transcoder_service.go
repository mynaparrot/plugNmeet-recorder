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

	"github.com/mynaparrot/plugnmeet-protocol/hooks"
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
						case <-time.After(30 * time.Second):
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

				task := new(plugnmeet.TranscodingTask)
				if err := proto.Unmarshal(msg.Data(), task); err != nil {
					logger.WithError(err).Errorln("Failed to unmarshal transcoding task, sending NAK")
					// If we can't even unmarshal, it's likely a bad message. Nak it with a delay.
					_ = msg.NakWithDelay(5 * time.Second)
					return
				}
				log := logger.WithFields(logrus.Fields{
					"recording_id":  task.RecordingId,
					"room_id":       task.RoomId,
					"room_sid":      task.RoomSid,
					"room_table_id": task.RoomTableId,
					"recorder_id":   task.RecorderId,
					"NumDelivered":  meta.NumDelivered,
				})

				if meta.NumDelivered > maxTranscodingRetries {
					log.Warnf("Transcoding job failed after %d attempts, removing from queue", maxTranscodingRetries)
					// Ack the message to prevent it from being redelivered
					_ = msg.Ack()
					continue
				}

				// All the tasks are a blocking call. The loop will not continue to the next Fetch until this transcoding is finished.
				started := time.Now()

				// Heartbeat: renew the ack deadline (msg.InProgress) every 15s so a
				// transcode longer than the consumer's AckWait isn't redelivered to a
				// second worker and processed twice. hbLog is a stable copy (the switch
				// reassigns log); hbStopped lets us wait for the goroutine to exit
				// before Ack/Nak so InProgress never races with them.
				hbLog := log
				hbDone := make(chan struct{})
				hbStopped := make(chan struct{})
				go func() {
					defer close(hbStopped)
					t := time.NewTicker(15 * time.Second)
					defer t.Stop()
					for {
						select {
						case <-hbDone:
							return
						case <-t.C:
							if err := msg.InProgress(); err != nil {
								hbLog.WithError(err).Warnln("Failed to send InProgress heartbeat")
							}
						}
					}
				}()

				// Wrap processing in a function to leverage defer for panic-safe cleanup.
				// This ensures the heartbeat goroutine is always stopped.
				procErr := func() error {
					defer func() {
						close(hbDone) // stop the heartbeat...
						<-hbStopped   // ...and wait for it to exit before Ack/Nak
					}()

					switch v := task.TaskDetails.(type) {
					case *plugnmeet.TranscodingTask_PostRecording:
						log = log.WithField("task", "post_recording")
						log.Info("Starting new 'post_recording' transcoding task")
						return c.handlePostRecordingTranscoding(task, v.PostRecording, log)
					case *plugnmeet.TranscodingTask_MergeRecordings:
						log = log.WithField("task", "merge_recordings")
						log.Info("Starting new 'merge_recordings' transcoding task")
						return c.handleMergeRecordings(task, v.MergeRecordings, log)
					}
					return nil
				}()

				if procErr != nil {
					log.WithError(procErr).Warnln("Transcoding failed, sending NAK to re-queue job")
					_ = msg.NakWithDelay(30 * time.Second)
				} else {
					// Everything was successful, ACK the message so it's not processed again.
					if err := msg.Ack(); err != nil {
						log.WithError(err).Errorln("Failed to send ACK for completed job")
					} else {
						log.Infof("Transcoding job completed and acknowledged after %s", time.Since(started))
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

	// Capture the initial remote path for potential cleanup after post-transcoding
	initialRemoteFilePath := taskDetails.FilePath

	// Run pre-transcoding scripts to get the file ready locally
	if len(c.cnf.Hooks.PreTranscoding) > 0 {
		var err error
		var preTranscodeHookResult *hooks.RecordingHookData
		taskDetails, preTranscodeHookResult, err = c.runPreTranscodingScripts(task, taskDetails, log)
		if err != nil {
			log.WithError(err).Errorln("Pre-transcoding script execution failed")
			return err
		}
		// If the hook returned a path that needs cleanup, defer the cleanup
		if preTranscodeHookResult != nil && preTranscodeHookResult.ShouldCleanup && preTranscodeHookResult.FilePath != "" {
			defer func(path string) {
				log.Infof("Cleaning up temporary directory/file: %s", path)
				if err := os.RemoveAll(path); err != nil {
					log.WithError(err).Errorf("Failed to clean up temporary directory/file: %s", path)
				}
			}(preTranscodeHookResult.FilePath)
		}
	}

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

	c.runPostTranscodingScriptsAndNotify(task, "single", toSend, outputFile, finalFileName, size, initialRemoteFilePath, log)
	log.Infoln("Post-transcoding process has been finished")

	return nil
}

func (c *RecorderController) handleMergeRecordings(task *plugnmeet.TranscodingTask, taskDetails *plugnmeet.TranscodingTaskMergeRecordings, log *logrus.Entry) error {
	if len(taskDetails.FilePaths) == 0 {
		log.Errorln("no file paths provided for merging")
		return errors.New("no file paths provided for merging")
	}

	// This is the base path where the transcoder expects to find the final merged file.
	// It's based on the original job request.
	relativeOutputDir := filepath.Dir(taskDetails.FilePaths[0])
	// This is the initial location where the transcoder will look for the source files.
	// It might be a network path.
	inputPath := path.Join(c.cnf.Recorder.CopyToPath.MainPath, relativeOutputDir)

	var preTranscodeHookResult *hooks.RecordingHookData
	if len(c.cnf.Hooks.PreTranscoding) > 0 {
		data := &hooks.RecordingHookData{
			Task:        "merge",
			RecordingID: task.GetRecordingId(),
			RoomTableID: task.GetRoomTableId(),
			RoomID:      task.GetRoomId(),
			RoomSID:     task.GetRoomSid(),
			FilePaths:   taskDetails.FilePaths,
			RecorderID:  task.GetRecorderId(),
		}

		jsonData, err := hooks.ExecuteHookPipeline(c.ctx, c.cnf.Hooks.PreTranscoding, data, log)
		if err != nil {
			return fmt.Errorf("pre-transcoding script execution failed for merge task: %w", err)
		}

		if len(jsonData) > 0 {
			var finalData hooks.RecordingHookData
			if err := json.Unmarshal(jsonData, &finalData); err != nil {
				log.WithError(err).Error("failed to unmarshal final JSON from pre-transcoding scripts for merge task, will use original data")
			} else {
				// The script returns the new local base path for the source files.
				if finalData.FilePath != "" {
					inputPath = finalData.FilePath
				}
				preTranscodeHookResult = &finalData // Capture the result for cleanup
			}
		}
	}

	// If the hook returned a path that needs cleanup, defer the cleanup
	if preTranscodeHookResult != nil && preTranscodeHookResult.ShouldCleanup && preTranscodeHookResult.FilePath != "" {
		defer func(path string) {
			log.Infof("Cleaning up temporary directory/file: %s", path)
			if err := os.RemoveAll(path); err != nil {
				log.WithError(err).Errorf("Failed to clean up temporary directory/file: %s", path)
			}
		}(preTranscodeHookResult.FilePath)
	}

	// The final output will still be placed relative to the original MainPath.
	fullOutputDir := path.Join(c.cnf.Recorder.CopyToPath.MainPath, relativeOutputDir)
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
		// Use the (potentially new) inputPath as the base for finding the source files.
		// We only need the base name of the original path `p`.
		abs, err := filepath.Abs(path.Join(inputPath, filepath.Base(p)))
		if err != nil {
			_ = file.Close()
			return err
		}

		// Check if file exists before adding to list
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			_ = file.Close()
			log.WithError(err).Errorf("file not found for merging: %s", abs)
			return err
		}

		// Write to the concat file for ffmpeg
		if _, err := file.WriteString(fmt.Sprintf("file '%s'\n", abs)); err != nil {
			log.WithError(err).Errorln("failed to write to ffmpeg file list")
			_ = file.Close()
			return err
		}
	}
	_ = file.Close() // Close the file so ffmpeg can read it

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
		Task:        plugnmeet.RecordingTasks_RECORDING_PROCEEDED,
		Msg:         "success",
		RecordingId: task.RecordingId,
		RecorderId:  task.RecorderId,
		RoomTableId: task.RoomTableId,
		FilePath:    relativePath,
		FileSize:    float32(math.Round(float64(size)*100) / 100),
	}
	c.runPostTranscodingScriptsAndNotify(task, "merge", toSend, outputFile, finalFileName, size, "", log) // No initial remote path for merge
	log.Infoln("merge recordings process has been finished")
	return nil
}

func (c *RecorderController) runPreTranscodingScripts(task *plugnmeet.TranscodingTask, taskDetails *plugnmeet.TranscodingTaskPostRecording, log *logrus.Entry) (*plugnmeet.TranscodingTaskPostRecording, *hooks.RecordingHookData, error) {
	data := &hooks.RecordingHookData{
		Task:        "single",
		RecordingID: task.GetRecordingId(),
		RoomTableID: task.GetRoomTableId(),
		RoomID:      task.GetRoomId(),
		RoomSID:     task.GetRoomSid(),
		FileName:    taskDetails.FileName,
		FilePath:    taskDetails.FilePath,
		RecorderID:  task.GetRecorderId(),
	}

	jsonData, err := hooks.ExecuteHookPipeline(c.ctx, c.cnf.Hooks.PreTranscoding, data, log)
	if err != nil {
		return nil, nil, err
	}

	var finalData hooks.RecordingHookData
	if len(jsonData) > 0 {
		if err := json.Unmarshal(jsonData, &finalData); err != nil {
			log.WithError(err).Error("failed to unmarshal final JSON from pre-transcoding scripts, will use original data")
			// In case of unmarshal error, we still return the original taskDetails and an empty finalData
			return taskDetails, &hooks.RecordingHookData{}, nil
		}

		if finalData.FilePath != "" {
			taskDetails.FilePath = finalData.FilePath
		}
		if finalData.FileName != "" {
			taskDetails.FileName = finalData.FileName
		}
	}

	return taskDetails, &finalData, nil
}

func (c *RecorderController) runPostTranscodingScriptsAndNotify(task *plugnmeet.TranscodingTask, taskType string, toSend *plugnmeet.RecorderToPlugNmeet, outputFile, finalFileName string, size float32, sourceForCleanup string, log *logrus.Entry) {
	if len(c.cnf.Hooks.PostTranscoding) > 0 {
		data := &hooks.RecordingHookData{
			Task:             taskType,
			RecordingID:      task.RecordingId,
			RoomTableID:      task.RoomTableId,
			RoomID:           task.RoomId,
			RoomSID:          task.RoomSid,
			FileName:         finalFileName,
			FilePath:         outputFile,
			FileSize:         size,
			RecorderID:       task.RecorderId,
			SourceForCleanup: sourceForCleanup,
		}

		jsonData, err := hooks.ExecuteHookPipeline(c.ctx, c.cnf.Hooks.PostTranscoding, data, log)
		if err != nil {
			log.WithError(err).Error("post-transcoding script execution failed")
		} else if len(jsonData) > 0 {
			var finalData hooks.RecordingHookData
			if err := json.Unmarshal(jsonData, &finalData); err == nil {
				if finalData.FilePath != "" {
					toSend.FilePath = finalData.FilePath
					log.Infof("post-transcoding script updated FilePath to: %s", finalData.FilePath)
				}
			} else {
				log.Errorf("failed to unmarshal post-transcoding script output, will use original data. output: %s", string(jsonData))
			}
		}
	}

	log.Infof("Notifying plugnmeet with data: %+v", toSend)
	if _, err := c.notifier.NotifyToPlugNmeet(toSend); err != nil {
		log.WithError(err).Errorln("failed to notify plugnmeet")
	}
}
