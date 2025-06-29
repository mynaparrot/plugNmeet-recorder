package natsservice

import (
	"errors"
	"fmt"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/nats-io/nats.go/jetstream"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

const (
	RecorderKvBucket   = "%s-%s"
	maxUpdateRetries   = 5
	retryUpdateBackoff = 50 * time.Millisecond
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
			log.Errorln(err)
		}
	}

	return nil
}

func (s *NatsService) UpdateLastPing() error {
	bucket := fmt.Sprintf(RecorderKvBucket, s.app.NatsInfo.Recorder.RecorderInfoKv, s.app.Recorder.Id)
	kv, err := s.js.KeyValue(s.ctx, bucket)
	switch {
	case errors.Is(err, jetstream.ErrBucketNotFound):
		return errors.New("this recorder was not found")
	case err != nil:
		return err
	}

	_, err = kv.PutString(s.ctx, fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_LAST_PING), fmt.Sprintf("%d", time.Now().UTC().UnixMilli()))
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsService) UpdateCurrentProgress(increment bool) error {
	bucket := fmt.Sprintf(RecorderKvBucket, s.app.NatsInfo.Recorder.RecorderInfoKv, s.app.Recorder.Id)
	kv, err := s.js.KeyValue(s.ctx, bucket)
	switch {
	case errors.Is(err, jetstream.ErrBucketNotFound):
		return errors.New("this recorder was not found")
	case err != nil:
		return err
	}

	key := fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_CURRENT_PROGRESS)

	for i := 0; i < maxUpdateRetries; i++ {
		entry, err := kv.Get(s.ctx, key)
		if err != nil {
			return err
		}

		currentProgress, err := strconv.ParseUint(string(entry.Value()), 10, 64)
		if err != nil {
			return err
		}

		if increment {
			currentProgress++
		} else {
			if currentProgress > 0 {
				currentProgress--
			}
		}

		newValue := fmt.Sprintf("%d", currentProgress)
		_, err = kv.Update(s.ctx, key, []byte(newValue), entry.Revision())
		if err == nil {
			return nil // Success
		}

		var apiErr *jetstream.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode == jetstream.JSErrCodeStreamWrongLastSequence {
			// This is the expected conflict error, wait a bit and retry.
			time.Sleep(retryUpdateBackoff)
			continue
		}

		// For any other error, return immediately.
		return fmt.Errorf("failed to update progress: %w", err)
	}

	return fmt.Errorf("failed to update progress after %d retries", maxUpdateRetries)
}
