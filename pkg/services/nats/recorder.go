package natsservice

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-protocol/utils"
	"github.com/nats-io/nats.go/jetstream"
)

var (
	maxLimitField = fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_MAX_LIMIT)
	progressField = fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_CURRENT_PROGRESS)
	lastPingField = fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_LAST_PING)
)

// AddRecorder initializes a recorder's information and ensures it is the only active instance with this ID.
func (s *NatsService) AddRecorder(pingInterval time.Duration) error {
	kv, err := s.js.KeyValue(s.ctx, s.app.NatsInfo.Recorder.RecorderInfoKv)
	if err != nil {
		return err
	}

	recorderId := s.app.Recorder.Id

	// Check if another instance is already active.
	lastPingKey := utils.FormatRecorderKey(recorderId, lastPingField)
	entry, err := kv.Get(s.ctx, lastPingKey)
	if err == nil && entry != nil {
		// Key exists, check if it's recent.
		lastPing, _ := strconv.ParseInt(string(entry.Value()), 10, 64)
		pingTime := time.UnixMilli(lastPing)
		cutoffTime := time.Now().UTC().Add(-pingInterval)

		if pingTime.After(cutoffTime) {
			// Another recorder is actively pinging. This instance should not run.
			return fmt.Errorf("another recorder with the same ID '%s' is already running. Exiting", recorderId)
		}
		// If the ping is stale, we can take over.
		s.logger.Warnf("Found stale entry for recorder ID '%s'. Taking over.", recorderId)
	} else if !errors.Is(err, jetstream.ErrKeyNotFound) {
		// An actual error occurred trying to get the key.
		return fmt.Errorf("failed to check for existing recorder: %w", err)
	}

	// It's safe to proceed. Write our own info.
	s.logger.Infof("Registering recorder with ID: %s", recorderId)
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

// UpdateLastPing updates the last ping timestamp for a specific recorder.
func (s *NatsService) UpdateLastPing() error {
	kv, err := s.js.KeyValue(s.ctx, s.app.NatsInfo.Recorder.RecorderInfoKv)
	if err != nil {
		return fmt.Errorf("recorder info bucket not found: %w", err)
	}

	recorderId := s.app.Recorder.Id
	key := utils.FormatRecorderKey(recorderId, lastPingField)

	_, err = kv.PutString(s.ctx, key, fmt.Sprintf("%d", time.Now().UTC().UnixMilli()))
	return err
}

// UpdateCurrentProgress updates the current progress for a specific recorder.
func (s *NatsService) UpdateCurrentProgress(progress int) error {
	kv, err := s.js.KeyValue(s.ctx, s.app.NatsInfo.Recorder.RecorderInfoKv)
	if err != nil {
		return fmt.Errorf("recorder info bucket not found: %w", err)
	}

	recorderId := s.app.Recorder.Id
	key := utils.FormatRecorderKey(recorderId, progressField)

	_, err = kv.PutString(s.ctx, key, fmt.Sprintf("%d", progress))
	return err
}

// DeleteRecorder removes all KV entries for this recorder to signal a graceful shutdown.
func (s *NatsService) DeleteRecorder() {
	kv, err := s.js.KeyValue(s.ctx, s.app.NatsInfo.Recorder.RecorderInfoKv)
	if err != nil {
		s.logger.WithError(err).Errorln("recorder info bucket not found")
		return
	}

	recorderId := s.app.Recorder.Id
	s.logger.Infof("performing graceful shutdown for recorder: %s", recorderId)

	// Use the shared variables to define all fields.
	fields := []string{
		maxLimitField,
		progressField,
		lastPingField,
	}

	// Purge each key. Purge sends a specific marker that watchers can act on.
	for _, field := range fields {
		key := utils.FormatRecorderKey(recorderId, field)
		err := kv.Purge(s.ctx, key)
		if err != nil {
			// Log the error but continue trying to delete other keys.
			s.logger.WithError(err).Errorf("failed to purge recorder key: %s", key)
		}
	}
}
