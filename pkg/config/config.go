package config

import (
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/mynaparrot/plugnmeet-protocol/hooks"
	"github.com/mynaparrot/plugnmeet-protocol/logging"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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
	NatsConn       *nats.Conn
	JetStream      jetstream.JetStream
	Logger         *logrus.Logger
	IsShuttingDown *atomic.Bool

	RootWorkingDir string
	Recorder       RecorderInfo        `yaml:"recorder"`
	Hooks          HooksConfig         `yaml:"hooks"`
	LogSettings    logging.LogSettings `yaml:"log_settings"`
	FfmpegSettings *FfmpegSettings     `yaml:"ffmpeg_settings"`
	NatsInfo       NatsInfo            `yaml:"nats_info"`
	PlugNmeetInfo  PlugNmeetInfo       `yaml:"plugNmeet_info"`
}

type HooksConfig struct {
	PostRecording   []string `yaml:"post_recording"`
	PreTranscoding  []string `yaml:"pre_transcoding"`
	PostTranscoding []string `yaml:"post_transcoding"`
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

func New(a *AppConfig) *AppConfig {
	a.setDefaultConfig()
	return a
}

func (a *AppConfig) setDefaultConfig() {
	a.IsShuttingDown = new(atomic.Bool)

	// Set default mode or validate the provided one.
	switch a.Recorder.Mode {
	case ModeBoth, ModeRecorderOnly, ModeTranscoderOnly:
		// valid mode, do nothing
	case "":
		// if not set, default to both
		a.Recorder.Mode = ModeBoth
	default:
		// if the value is not recognized, exit
		logrus.Fatalf("Invalid value for -mode flag: '%s'. Allowed values are 'both', 'recorderOnly', 'transcoderOnly'.", a.Recorder.Mode)
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

	// For backward compatibility, we'll show error message if user still using old format
	if len(a.Recorder.PostProcessingScripts) > 0 {
		logrus.Fatalf("Configuration error: 'recorder.post_processing_scripts' is deprecated. Please move your scripts to the top-level 'hooks.post_transcoding' section. IMPORTANT: Scripts now receive data via stdin instead of command-line arguments. Please update your scripts to read from stdin.")
	}

	// Validate all defined hook scripts
	if a.Recorder.Mode == ModeBoth || a.Recorder.Mode == ModeRecorderOnly {
		for i, script := range a.Hooks.PostRecording {
			resolved := a.resolvePath(script)
			a.Hooks.PostRecording[i] = resolved
			if err := hooks.ValidateHookScript(resolved, "post_recording"); err != nil {
				logrus.WithError(err).Fatal("Error validating hook script")
			}
		}
	}

	if a.Recorder.Mode == ModeBoth || a.Recorder.Mode == ModeTranscoderOnly {
		for i, script := range a.Hooks.PreTranscoding {
			resolved := a.resolvePath(script)
			a.Hooks.PostRecording[i] = resolved
			if err := hooks.ValidateHookScript(resolved, "pre_transcoding"); err != nil {
				logrus.WithError(err).Fatal("Error validating hook script")
			}
		}

		for i, script := range a.Hooks.PostTranscoding {
			resolved := a.resolvePath(script)
			a.Hooks.PostRecording[i] = resolved
			if err := hooks.ValidateHookScript(resolved, "post_transcoding"); err != nil {
				logrus.WithError(err).Fatal("Error validating hook script")
			}
		}
	}

	if a.FfmpegSettings == nil {
		a.FfmpegSettings = &FfmpegSettings{
			Recording: FfmpegOptions{
				PreInput:  "-loglevel error -thread_queue_size 512 -draw_mouse 0 -threads 1",
				PostInput: "-c:v libx264 -x264-params keyint=120:scenecut=0 -preset ultrafast -crf 23 -c:a copy -movflags frag_keyframe+empty_moov+default_base_moof -flush_packets 1 -tune zerolatency -y",
			},
			PostRecording: FfmpegOptions{
				PreInput:  "-loglevel error",
				PostInput: "-pix_fmt yuv420p -preset veryfast -movflags faststart -y -c:a aac -af highpass=f=200,lowpass=f=4000,afftdn",
			},
			Rtmp: FfmpegOptions{
				PreInput:  "-loglevel error -draw_mouse 0 -threads 1",
				PostInput: "-c:v libx264 -pix_fmt yuv420p -x264-params keyint=120:scenecut=0 -b:v 2500k -video_size 1920x1080 -c:a aac -b:a 128k -ar 44100 -preset ultrafast -crf 23 -movflags frag_keyframe+empty_moov+default_base_moof -bufsize 5000k -flush_packets 1 -tune zerolatency -f flv",
			},
		}
	}
	if a.NatsInfo.Recorder.TranscodingJobs == "" {
		a.NatsInfo.Recorder.TranscodingJobs = "pnm-RecorderTranscoderJobs"
	}
}

func (a *AppConfig) resolvePath(scriptPath string) string {
	if strings.HasPrefix(scriptPath, "./") {
		return filepath.Join(a.RootWorkingDir, scriptPath)
	}
	return scriptPath
}
