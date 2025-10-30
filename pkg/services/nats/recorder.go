package natsservice

import (
	"errors"
	"fmt"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	RecorderKvBucket = "%s-%s"
)

func (s *NatsService) AddRecorder() error {
	bucket := fmt.Sprintf(RecorderKvBucket, s.app.NatsInfo.Recorder.RecorderInfoKv, s.app.Recorder.Id)
	kv, err := s.js.CreateOrUpdateKeyValue(s.ctx, jetstream.KeyValueConfig{
		Replicas: s.app.NatsInfo.NumReplicas,
		Bucket:   bucket,
	})
	if err != nil {
		return err
	}

	data := map[string]string{
		fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_MAX_LIMIT):        fmt.Sprintf("%d", s.app.Recorder.MaxLimit),
		fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_CURRENT_PROGRESS): "0",
		fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_LAST_PING):        fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
	}

	for k, v := range data {
		_, err = kv.PutString(s.ctx, k, v)
		if err != nil {
			s.logger.WithError(err).Errorln("failed to add recorder info")
		}
	}

	return nil
}

func (s *NatsService) UpdateLastPing() error {
	bucket := fmt.Sprintf(RecorderKvBucket, s.app.NatsInfo.Recorder.RecorderInfoKv, s.app.Recorder.Id)
	kv, err := s.js.KeyValue(s.ctx, bucket)
	switch {
	case errors.Is(err, jetstream.ErrBucketNotFound):
		return fmt.Errorf("this recorder was not found")
	case err != nil:
		return err
	}

	_, err = kv.PutString(s.ctx, fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_LAST_PING), fmt.Sprintf("%d", time.Now().UTC().UnixMilli()))
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsService) UpdateCurrentProgress(progress int) error {
	bucket := fmt.Sprintf(RecorderKvBucket, s.app.NatsInfo.Recorder.RecorderInfoKv, s.app.Recorder.Id)
	kv, err := s.js.KeyValue(s.ctx, bucket)
	switch {
	case errors.Is(err, jetstream.ErrBucketNotFound):
		return fmt.Errorf("this recorder was not found")
	case err != nil:
		return err
	}

	key := fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_CURRENT_PROGRESS)
	newValue := fmt.Sprintf("%d", progress)

	_, err = kv.PutString(s.ctx, key, newValue)
	return err
}
