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
	"github.com/nats-io/nats.go/jetstream"
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

	logger.Infoln("Transcoding service started successfully")

	// Single loop ensures that only one job is processed at a time by this worker instance.
	for {
		select {
		case <-c.ctx.Done():
			logger.Infoln("Closing transcoding worker")
			return
		default:
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

				// This is a blocking call. The loop will not continue
				// to the next Fetch until this transcoding is finished.
				c.handleTranscoding(msg, logger)
			}
		}
	}
}

func (c *RecorderController) handleTranscoding(msg jetstream.Msg, logger *logrus.Entry) {
	task := new(plugnmeet.TranscodingTask)
	if err := proto.Unmarshal(msg.Data(), task); err != nil {
		logger.WithError(err).Errorln("Failed to unmarshal transcoding task, sending NAK")
		// If we can't even unmarshal, it's likely a bad message. Nak it with a delay.
		_ = msg.NakWithDelay(5 * time.Second)
		return
	}

	log := logger.WithFields(logrus.Fields{
		"recordingId": task.RecordingId,
		"roomId":      task.RoomId,
		"filePath":    task.FilePath,
		"fileName":    task.FileName,
		"method":      "handleTranscoding",
	})

	// Use a deferred function to ensure the message is always NAK'd if not explicitly ACK'd.
	acked := false
	defer func() {
		if !acked {
			// If we haven't acked by the end, something went wrong. Nak it.
			log.Warnln("Transcoding failed or not acknowledged, sending NAK to re-queue job")
			_ = msg.NakWithDelay(10 * time.Second) // Re-queue with a delay
		}
	}()

	rawFilePath := path.Join(task.FilePath, task.FileName)
	finalFileName := fmt.Sprintf("%s.mp4", task.RecordingId)
	outputFile := path.Join(task.FilePath, finalFileName)

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
		_ = msg.Ack()
		acked = true
		return
	}

	if c.cnf.Recorder.PostMp4Convert {
		preArgs, err := shell.Fields(c.cnf.FfmpegSettings.PostRecording.PreInput, nil)
		if err != nil {
			log.WithError(err).Errorln("Failed to parse ffmpeg pre-input args")
			return // will be NAK'd
		}
		postArgs, err := shell.Fields(c.cnf.FfmpegSettings.PostRecording.PostInput, nil)
		if err != nil {
			log.WithError(err).Errorln("Filed to parse ffmpeg post-input args")
			return // will be NAK'd
		}

		args := append(preArgs, "-i", rawFilePath)
		args = append(args, postArgs...)
		args = append(args, outputFile)
		log.Infof("Starting post recording ffmpeg process with args: %s", strings.Join(args, " "))

		cmd := exec.CommandContext(c.ctx, "ffmpeg", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.WithError(err).Errorf("Transcoding (ffmpeg) failed. Keeping raw file: %s as output because of error. Output: %s", task.FileName, string(output))
			// remove the new file if it was partially created
			_ = os.Remove(outputFile)
			// keep the old file as output by setting finalFileName to raw file
			finalFileName = task.FileName
			outputFile = rawFilePath
			return // Deferred NAK will handle re-queueing
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
			log.WithError(err).Errorf("Keeping the raw file: %s as output because of error during rename: %s", task.FileName, err.Error())
			// keep the old file as output
			finalFileName = task.FileName
			outputFile = rawFilePath
			return // Deferred NAK will handle re-queueing
		}
		log.Infoln("Raw file renamed to final output file")
	}

	// Calculate file size and relative path for notification
	stat, err := os.Stat(outputFile)
	if err != nil {
		log.WithError(err).Errorln("Failed to stat final output file after processing")
		return // Deferred NAK will handle re-queueing
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
		RecordingVariant: &task.RecordingVariant,
	}

	log.Infof("Notifying plugnmeet with data: %+v", toSend)

	if _, err = c.notifier.NotifyToPlugNmeet(toSend); err != nil {
		log.WithError(err).Errorln("Failed to notify plugnmeet")
		// This is a critical failure, but the file is processed. We still ACK the NATS message.
	}

	// Execute post-processing scripts
	if len(c.cnf.Recorder.PostProcessingScripts) > 0 {
		data := map[string]interface{}{
			"recording_id":  task.GetRecordingId(),
			"room_table_id": task.GetRoomTableId(),
			"room_id":       task.GetRoomId(),
			"room_sid":      task.GetRoomSid(),
			"file_name":     finalFileName,
			"file_path":     outputFile, // this will be the full path of the final file
			"file_size":     size,
			"recorder_id":   task.GetRecorderId(),
		}
		marshal, err := json.Marshal(data)
		if err != nil {
			log.WithError(err).Errorln("Failed to marshal post-processing data for scripts")
		} else {
			for _, script := range c.cnf.Recorder.PostProcessingScripts {
				log.Infof("running post-processing script: %s", script)
				cmd := exec.Command("/bin/sh", script, string(marshal))
				scriptOutput, scriptErr := cmd.CombinedOutput()
				if scriptErr != nil {
					log.WithError(scriptErr).Errorf("Post-processing script failed: %s, Output: %s", script, string(scriptOutput))
				} else {
					log.Infof("Post-processing script %s finished. Output: %s", script, string(scriptOutput))
				}
			}
		}
	}
	log.Infoln("Post process recording has been finished")

	// Everything was successful, ACK the message so it's not processed again.
	if err := msg.Ack(); err != nil {
		log.WithError(err).Errorln("Failed to send ACK for completed job")
	} else {
		acked = true // Mark as acked
		log.Infoln("Transcoding job completed and acknowledged")
	}
}
