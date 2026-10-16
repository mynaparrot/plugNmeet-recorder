package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// RetryTranscode re-submits a post-recording transcoding job that the transcoder
// consumer dropped after exhausting its delivery attempts. It connects to NATS with
// the recorder's own credentials and publishes a fresh job to the transcoding subject.
func RetryTranscode(args []string) error {
	fs := flag.NewFlagSet("retry-transcode", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		configPath  string
		recordingId string
		roomTableId int64
		roomId      string
		file        string
	)

	fs.StringVar(&configPath, "config", "config.yaml", "Path to the configuration file")
	fs.StringVar(&recordingId, "recording-id", "", "Recording ID of the job to re-submit")
	fs.Int64Var(&roomTableId, "room-table-id", 0, "Room table ID (the roomTableId field in recorder logs)")
	fs.StringVar(&roomId, "room-id", "", "Room ID")
	fs.StringVar(&file, "file", "", "Full path to the raw recording file (must be resolvable by the transcoder)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `Re-submit a post-recording transcoding job that NATS dropped.

Usage:
  %s retry-transcode --recording-id <id> --room-table-id <id> --room-id <id> --file <path> [flags]

Example:
  %s retry-transcode --recording-id rec-123 --room-table-id 42 --room-id room-abc --file /recordings/room-abc/rec-123_raw.mp4

Flags:
`, os.Args[0], os.Args[0])
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	var missing []string
	if recordingId == "" {
		missing = append(missing, "recording-id")
	}
	if roomTableId == 0 {
		missing = append(missing, "room-table-id")
	}
	if roomId == "" {
		missing = append(missing, "room-id")
	}
	if file == "" {
		missing = append(missing, "file")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flag(s): %s", strings.Join(missing, ", "))
	}

	appCnf, err := provideAppConfig(configPath, "")
	if err != nil {
		return err
	}

	task := &plugnmeet.TranscodingTask{
		RecordingId: recordingId,
		RoomId:      roomId,
		RoomTableId: roomTableId,
		RecorderId:  appCnf.Recorder.Id,
		TaskDetails: &plugnmeet.TranscodingTask_PostRecording{
			PostRecording: &plugnmeet.TranscodingTaskPostRecording{
				FilePath: file,
			},
		},
	}

	data, err := proto.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal transcoding task: %w", err)
	}

	logger := logrus.New()
	nc, err := connectNATS(appCnf, logger, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = nc.Drain()
	}()

	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("failed to create jetstream context: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"recordingId": recordingId,
		"roomTableId": roomTableId,
		"file":        file,
	}).Info("Publishing retry transcoding task")

	ack, err := js.Publish(context.Background(), appCnf.NatsInfo.Recorder.TranscodingJobs, data)
	if err != nil {
		return fmt.Errorf("failed to publish transcoding task to %s: %w", appCnf.NatsInfo.Recorder.TranscodingJobs, err)
	}

	logger.WithFields(logrus.Fields{
		"subject":  appCnf.NatsInfo.Recorder.TranscodingJobs,
		"stream":   ack.Stream,
		"sequence": ack.Sequence,
	}).Info("Retry transcoding task published successfully")

	return nil
}
