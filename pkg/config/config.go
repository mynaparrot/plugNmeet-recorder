package config

import (
	"fmt"
	"path/filepath"

	"github.com/mynaparrot/plugnmeet-protocol/hooks"
	"github.com/mynaparrot/plugnmeet-protocol/logging"
	"github.com/sirupsen/logrus"
)

// AppMode defines the application's operating mode.
type AppMode string

const (
	ModeRecorderOnly   AppMode = "recorderOnly"   // only recording and RTMP broadcasting
	ModeTranscoderOnly AppMode = "transcoderOnly" // only post-processing files e.g. transcoding in MP4
	ModeBoth           AppMode = "both"           // both recording/RTMP and post-processing
)

type AppConfig struct {
	RootWorkingDir string

	Recorder       RecorderInfo        `yaml:"recorder"`
	Hooks          *Hooks              `yaml:"hooks"`
	LogSettings    logging.LogSettings `yaml:"log_settings"`
	FfmpegSettings *FfmpegSettings     `yaml:"ffmpeg_settings"`
	NatsInfo       NatsInfo            `yaml:"nats_info"`
	PlugNmeetInfo  PlugNmeetInfo       `yaml:"plugNmeet_info"`
}

type RecorderInfo struct {
	Id                          string             `yaml:"id"`
	Mode                        AppMode            `yaml:"mode"`
	MaxLimit                    uint64             `yaml:"max_limit"`
	Debug                       bool               `yaml:"debug"`
	PostMp4Convert              bool               `yaml:"post_mp4_convert"`
	CustomChromePath            *string            `yaml:"custom_chrome_path"`
	Width                       uint64             `yaml:"width"`
	Height                      uint64             `yaml:"height"`
	XvfbDpi                     uint64             `yaml:"xvfb_dpi"`
	TemporaryDir                *string            `yaml:"temporary_dir"`
	CopyToPath                  CopyToPathSettings `yaml:"copy_to_path"`
	TranscodingCpuLimitBothMode *float64           `yaml:"transcoding_cpu_limit_both_mode"`

	// Deprecated fields for backward compatibility. They are unexported and will be migrated
	// to the top-level `hooks` section during config initialization.
	PostProcessingScripts []string `yaml:"post_processing_scripts"` // Deprecated
}

type CopyToPathSettings struct {
	MainPath string `yaml:"main_path"`
	SubPath  string `yaml:"sub_path"`
}

type FfmpegSettings struct {
	Recording     FfmpegOptions `yaml:"recording"`
	PostRecording FfmpegOptions `yaml:"post_recording"`
	Rtmp          FfmpegOptions `yaml:"rtmp"`
}

type FfmpegOptions struct {
	PreInput  string `yaml:"pre_input"`
	PostInput string `yaml:"post_input"`
}

type PlugNmeetInfo struct {
	Host      string  `yaml:"host"`
	ApiKey    string  `yaml:"api_key"`
	ApiSecret string  `yaml:"api_secret"`
	JoinHost  *string `yaml:"join_host"`
}

type NatsInfo struct {
	NatsUrls    []string         `yaml:"nats_urls"`
	NumReplicas int              `yaml:"num_replicas"`
	Recorder    NatsInfoRecorder `yaml:"recorder"`
}

type NatsInfoRecorder struct {
	RecorderChannel string `yaml:"recorder_channel"`
	RecorderInfoKv  string `yaml:"recorder_info_kv"`
	TranscodingJobs string `yaml:"transcoding_jobs_subject"`
}

func Initialize(a *AppConfig) (*AppConfig, error) {
	if err := a.setDefaultConfig(); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *AppConfig) setDefaultConfig() error {
	// Set default mode or validate the provided one.
	switch a.Recorder.Mode {
	case ModeBoth, ModeRecorderOnly, ModeTranscoderOnly:
		// valid mode, do nothing
	case "":
		// if not set, default to both
		a.Recorder.Mode = ModeBoth
	default:
		// if the value is not recognized, exit
		return fmt.Errorf("invalid value for -mode flag: '%s'. Allowed values are 'both', 'recorderOnly', 'transcoderOnly'", a.Recorder.Mode)
	}

	if a.Recorder.MaxLimit == 0 {
		a.Recorder.MaxLimit = 10
	}
	if a.Recorder.Width == 0 {
		a.Recorder.Width = 1920
	}
	if a.Recorder.Height == 0 {
		a.Recorder.Height = 1080
	}
	if a.Recorder.XvfbDpi == 0 {
		a.Recorder.XvfbDpi = 96
	}
	if a.Recorder.TranscodingCpuLimitBothMode == nil {
		a.Recorder.TranscodingCpuLimitBothMode = new(80.0)
	}

	if !filepath.IsAbs(a.LogSettings.LogFile) {
		a.LogSettings.LogFile = filepath.Join(a.RootWorkingDir, a.LogSettings.LogFile)
	}

	// For backward compatibility
	if len(a.Recorder.PostProcessingScripts) > 0 {
		logrus.Warnln("Configuration error: 'recorder.post_processing_scripts' is deprecated. Please move your scripts to the top-level 'hooks.post_transcoding' section. IMPORTANT: Scripts now receive data via stdin instead of command-line arguments. Please update your scripts to read from stdin.")

		if a.Hooks == nil {
			a.Hooks = new(Hooks)
		}

		if a.Hooks.PostTranscoding == nil {
			a.Hooks.PostTranscoding = new(hooks.HookScriptConfig)
			for _, script := range a.Recorder.PostProcessingScripts {
				a.Hooks.PostTranscoding.Scripts = append(a.Hooks.PostTranscoding.Scripts, hooks.HookScript{Script: script, IsOneShot: true})
			}
		}
	}

	if a.FfmpegSettings == nil {
		a.FfmpegSettings = &FfmpegSettings{
			Recording: FfmpegOptions{
				PreInput:  "-loglevel error -thread_queue_size 1024 -framerate 30 -draw_mouse 0 -threads 1",
				PostInput: "-c:v libx264 -pix_fmt yuv420p -bf 0 -x264-params keyint=120:scenecut=0 -preset ultrafast -crf 23 -c:a copy -movflags frag_keyframe+empty_moov+default_base_moof -flush_packets 1 -tune zerolatency -y",
			},
			PostRecording: FfmpegOptions{
				PreInput:  "-loglevel error",
				PostInput: "-c:v libx264 -profile:v high -level:v 4.1 -pix_fmt yuv420p -preset medium -crf 20 -movflags faststart -y -c:a aac -b:a 128k -af highpass=f=80,lowpass=f=8000,afftdn",
			},
			Rtmp: FfmpegOptions{
				PreInput:  "-loglevel error -thread_queue_size 512 -framerate 30 -draw_mouse 0 -threads 1",
				PostInput: "-c:v libx264 -profile:v baseline -pix_fmt yuv420p -bf 0 -x264-params keyint=60:scenecut=0:nal-hrd=cbr -b:v 2500k -maxrate 2500k -bufsize 2500k -video_size 1920x1080 -c:a aac -b:a 128k -ar 44100 -preset ultrafast -tune zerolatency -flush_packets 1 -f flv",
			},
		}
	}
	if a.NatsInfo.Recorder.TranscodingJobs == "" {
		a.NatsInfo.Recorder.TranscodingJobs = "pnm-RecorderTranscoderJobs"
	}

	return nil
}
