package natsservice

import (
	"fmt"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-protocol/utils"
)

// AddRecorder initializes a recorder's information in the consolidated KV bucket.
func (s *NatsService) AddRecorder() error {
	kv, err := s.js.KeyValue(s.ctx, s.app.NatsInfo.Recorder.RecorderInfoKv)
	if err != nil {
		return err
	}

	recorderId := s.app.Recorder.Id
	maxLimitField := fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_MAX_LIMIT)
	progressField := fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_CURRENT_PROGRESS)
	lastPingField := fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_LAST_PING)

	// Prepare data using the new key format from the utils package
	data := map[string]string{
		utils.FormatRecorderKey(recorderId, maxLimitField): fmt.Sprintf("%d", s.app.Recorder.MaxLimit),
		utils.FormatRecorderKey(recorderId, progressField): "0",
		utils.FormatRecorderKey(recorderId, lastPingField): fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
	}

	// Put all fields into the consolidated bucket
	for k, v := range data {
		_, err = kv.PutString(s.ctx, k, v)
		if err != nil {
			s.logger.WithError(err).Errorln("failed to add recorder info field")
		}
	}

	return nil
}

// UpdateLastPing updates the last ping timestamp for a specific recorder in the consolidated KV bucket.
func (s *NatsService) UpdateLastPing() error {
	kv, err := s.js.KeyValue(s.ctx, s.app.NatsInfo.Recorder.RecorderInfoKv)
	if err != nil {
		return fmt.Errorf("recorder info bucket not found: %w", err)
	}

	recorderId := s.app.Recorder.Id
	lastPingField := fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_LAST_PING)
	key := utils.FormatRecorderKey(recorderId, lastPingField)

	_, err = kv.PutString(s.ctx, key, fmt.Sprintf("%d", time.Now().UTC().UnixMilli()))
	return err
}

// UpdateCurrentProgress updates the current progress for a specific recorder in the consolidated KV bucket.
func (s *NatsService) UpdateCurrentProgress(progress int) error {
	kv, err := s.js.KeyValue(s.ctx, s.app.NatsInfo.Recorder.RecorderInfoKv)
	if err != nil {
		return fmt.Errorf("recorder info bucket not found: %w", err)
	}

	recorderId := s.app.Recorder.Id
	progressField := fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_CURRENT_PROGRESS)
	key := utils.FormatRecorderKey(recorderId, progressField)

	_, err = kv.PutString(s.ctx, key, fmt.Sprintf("%d", progress))
	return err
}
