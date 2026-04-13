package natsservice

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-protocol/utils"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/shirou/gopsutil/v4/cpu"
)

var (
	maxLimitField   = fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_MAX_LIMIT)
	progressField   = fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_CURRENT_PROGRESS)
	lastPingField   = fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_LAST_PING)
	totalCoresField = fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_TOTAL_CORES)
	cpuScoreField   = fmt.Sprintf("%d", plugnmeet.RecorderInfoKeys_RECORDER_INFO_CPU_SCORE)
)

// RegisterAsActiveRecorder initializes a recorder's information and ensures it is the only active instance with this ID.
func (s *NatsService) RegisterAsActiveRecorder(pingInterval time.Duration) error {
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
	data := map[string]string{
		utils.FormatRecorderKey(recorderId, maxLimitField):   fmt.Sprintf("%d", s.app.Recorder.MaxLimit),
		utils.FormatRecorderKey(recorderId, progressField):   "0",
		utils.FormatRecorderKey(recorderId, lastPingField):   fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		utils.FormatRecorderKey(recorderId, totalCoresField): fmt.Sprintf("%d", runtime.NumCPU()),
	}

	// Put all fields into the consolidated bucket
	for k, v := range data {
		if _, err = kv.PutString(s.ctx, k, v); err != nil {
			s.logger.WithError(err).Errorln("failed to add recorder info field")
		}
	}

	s.logger.Infof("Successfully registered recorder with ID: %s", recorderId)
	return nil
}

// UpdateStatus updates the last ping timestamp and current system load for current recorder.
func (s *NatsService) UpdateStatus() error {
	kv, err := s.js.KeyValue(s.ctx, s.app.NatsInfo.Recorder.RecorderInfoKv)
	if err != nil {
		return fmt.Errorf("recorder info bucket not found: %w", err)
	}

	recorderId := s.app.Recorder.Id

	// Update last ping
	pingKey := utils.FormatRecorderKey(recorderId, lastPingField)
	_, err = kv.PutString(s.ctx, pingKey, fmt.Sprintf("%d", time.Now().UTC().UnixMilli()))
	if err != nil {
		return err
	}

	// Calculate and update CPU score
	cpuPercentages, err := cpu.Percent(time.Second, false)
	if err != nil {
		s.logger.WithError(err).Error("failed to get cpu usage")
	} else if len(cpuPercentages) > 0 {
		cpuScore := cpuPercentages[0] / 100 // Normalize to 0-1
		cpuKey := utils.FormatRecorderKey(recorderId, cpuScoreField)
		if _, err = kv.PutString(s.ctx, cpuKey, fmt.Sprintf("%.4f", cpuScore)); err != nil {
			return err
		}
	}

	return nil
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
		totalCoresField,
		cpuScoreField,
	}

	// Purge each key. Purge sends a specific marker that watchers can act on.
	for _, field := range fields {
		key := utils.FormatRecorderKey(recorderId, field)
		if err := kv.Purge(s.ctx, key); err != nil {
			// Log the error but continue trying to delete other keys.
			s.logger.WithError(err).Errorf("failed to purge recorder key: %s", key)
		}
	}
}
