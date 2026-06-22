package config

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/hooks"
	log "github.com/sirupsen/logrus"
)

type Hooks struct {
	hookManager *hooks.HookProcessManager

	PostRecording   *hooks.HookScriptConfig `yaml:"post_recording"`
	PreTranscoding  *hooks.HookScriptConfig `yaml:"pre_transcoding"`
	PostTranscoding *hooks.HookScriptConfig `yaml:"post_transcoding"`
}

func (h *Hooks) InitializeStorageHooks(ctx context.Context, appCnf *AppConfig, logger *log.Logger) error {
	scriptsWithPoolSize := make(map[string]int)

	resolvePath := func(scriptPath string) string {
		if !filepath.IsAbs(scriptPath) {
			scriptPath = filepath.Join(appCnf.RootWorkingDir, scriptPath)
		}
		return filepath.Clean(scriptPath)
	}

	processHookCategory := func(config *hooks.HookScriptConfig, name string) error {
		if config == nil {
			return nil
		}
		if config.PoolSize <= 0 {
			config.PoolSize = 1
		}
		if config.HookTimeout == 0 {
			config.HookTimeout = time.Hour // Recorder hooks can be long-running.
		}
		for i, script := range config.Scripts {
			var resolved string
			if strings.HasPrefix(script.Script, hooks.HookCommandHttpRequest) {
				resolved = script.Script
			} else {
				resolved = resolvePath(script.Script)
			}

			if err := hooks.ValidateHookScript(resolved, name); err != nil {
				return err
			}
			config.Scripts[i].Script = resolved

			if !script.IsOneShot {
				if currentSize, ok := scriptsWithPoolSize[resolved]; !ok || config.PoolSize > currentSize {
					scriptsWithPoolSize[resolved] = config.PoolSize
				}
			}
		}
		return nil
	}

	if appCnf.Recorder.Mode == ModeBoth || appCnf.Recorder.Mode == ModeRecorderOnly {
		if err := processHookCategory(h.PostRecording, "post_recording"); err != nil {
			return err
		}
	}

	if appCnf.Recorder.Mode == ModeBoth || appCnf.Recorder.Mode == ModeTranscoderOnly {
		if err := processHookCategory(h.PreTranscoding, "pre_transcoding"); err != nil {
			return err
		}
		if err := processHookCategory(h.PostTranscoding, "post_transcoding"); err != nil {
			return err
		}
	}

	// Initialize the HookProcessManager and start all unique scripts
	h.hookManager = hooks.NewHookProcessManager(ctx, logger.WithField("service", "hook_manager"))
	if len(scriptsWithPoolSize) > 0 {
		if err := h.hookManager.StartHookProcesses(scriptsWithPoolSize); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) RunPostRecordingHook(req *hooks.RecordingHookData, log *log.Entry) (*hooks.RecordingHookData, error) {
	if h.PostRecording == nil || len(h.PostRecording.Scripts) == 0 {
		return nil, nil
	}

	jsonData, err := hooks.ExecuteHookPipeline(h.hookManager, h.PostRecording.Scripts, req, h.PostRecording.HookTimeout, log)
	if err != nil {
		return nil, err
	}

	var finalData hooks.RecordingHookData
	if err := json.Unmarshal(jsonData, &finalData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal final JSON from post-recording scripts, will use original data")
	}
	if finalData.Error != "" {
		return nil, fmt.Errorf("post-recording script responded with error msg: %s", finalData.Error)
	}

	return &finalData, nil
}

func (h *Hooks) RunPreTranscodingHook(req *hooks.RecordingHookData, log *log.Entry) (*hooks.RecordingHookData, error) {
	if h.PreTranscoding == nil || len(h.PreTranscoding.Scripts) == 0 {
		return nil, nil
	}

	jsonData, err := hooks.ExecuteHookPipeline(h.hookManager, h.PreTranscoding.Scripts, req, h.PreTranscoding.HookTimeout, log)
	if err != nil {
		return nil, err
	}

	var finalData hooks.RecordingHookData
	if err := json.Unmarshal(jsonData, &finalData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal final JSON from pre-transcoding scripts, will use original data")
	}
	if finalData.Error != "" {
		return nil, fmt.Errorf("pre-transcoding script responded with error msg: %s", finalData.Error)
	}

	return &finalData, nil
}

func (h *Hooks) RunPostTranscodingHook(req *hooks.RecordingHookData, log *log.Entry) (*hooks.RecordingHookData, error) {
	if h.PostTranscoding == nil || len(h.PostTranscoding.Scripts) == 0 {
		return nil, nil
	}

	jsonData, err := hooks.ExecuteHookPipeline(h.hookManager, h.PostTranscoding.Scripts, req, h.PostTranscoding.HookTimeout, log)
	if err != nil {
		return nil, err
	}

	var finalData hooks.RecordingHookData
	if err := json.Unmarshal(jsonData, &finalData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal final JSON from post-transcoding scripts, will use original data")
	}
	if finalData.Error != "" {
		return nil, fmt.Errorf("post-transcoding script responded with error msg: %s", finalData.Error)
	}

	return &finalData, nil
}
