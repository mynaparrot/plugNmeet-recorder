package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// ScriptData defines the JSON payload passed to and from external scripts.
type ScriptData struct {
	Task        string   `json:"task,omitempty"` // e.g., "single", "merge"
	RecordingID string   `json:"recording_id"`
	RoomTableID int64    `json:"room_table_id"`
	RoomID      string   `json:"room_id"`
	RoomSID     string   `json:"room_sid"`
	FileName    string   `json:"file_name,omitempty"`  // For single file tasks
	FilePath    string   `json:"file_path,omitempty"`  // For single file tasks or as output
	FilePaths   []string `json:"file_paths,omitempty"` // For merge tasks
	RecorderID  string   `json:"recorder_id"`
	FileSize    float32  `json:"file_size,omitempty"` // Used in post-transcoding scripts
}

// RunScriptsWithData executes a series of scripts, passing data from one to the next via stdin/stdout.
// It's used for script stages that need to modify the job payload.
func RunScriptsWithData(ctx context.Context, scripts []string, initialData *ScriptData, log *logrus.Entry, scriptType string) (json.RawMessage, error) {
	jsonData, err := json.Marshal(initialData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initial data for %s script: %w", scriptType, err)
	}

	for _, script := range scripts {
		log.Infof("Running %s script: %s", scriptType, script)

		cmd := exec.CommandContext(ctx, script)
		cmd.Stdin = bytes.NewReader(jsonData)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("%s script %s failed: %w, stderr: %s", scriptType, script, err, stderr.String())
		}

		// The output of the script becomes the input for the next one.
		jsonData = out.Bytes()
	}

	return jsonData, nil
}

// RunFireAndForgetScripts executes a series of scripts for notification or cleanup purposes.
// It passes data as a command-line argument and does not process stdout.
func RunFireAndForgetScripts(ctx context.Context, scripts []string, data *ScriptData, log *logrus.Entry, scriptType string) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.WithError(err).Errorf("failed to marshal data for %s scripts", scriptType)
		return
	}

	for _, script := range scripts {
		log.Infof("Running %s script: %s", scriptType, script)

		cmd := exec.CommandContext(ctx, script, string(jsonData))
		output, err := cmd.CombinedOutput()

		if err != nil {
			log.WithError(err).Errorf("%s script %s failed, output: %s", scriptType, script, string(output))
		} else {
			log.Infof("%s script %s finished successfully. output: %s", scriptType, script, string(output))
		}
	}
}
