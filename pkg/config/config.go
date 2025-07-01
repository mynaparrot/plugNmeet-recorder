package config

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type AppConfig struct {
	NatsConn  *nats.Conn
	JetStream jetstream.JetStream

	RootWorkingDir string
	Recorder       RecorderInfo    `yaml:"recorder"`
	LogSettings    LogSettings     `yaml:"log_settings"`
	FfmpegSettings *FfmpegSettings `yaml:"ffmpeg_settings"`
	NatsInfo       NatsInfo        `yaml:"nats_info"`
	PlugNmeetInfo  PlugNmeetInfo   `yaml:"plugNmeet_info"`
}

type RecorderInfo struct {
	Id                    string             `yaml:"id"`
	MaxLimit              uint64             `yaml:"max_limit"`
	Debug                 bool               `yaml:"debug"`
	PostMp4Convert        bool               `yaml:"post_mp4_convert"`
	CustomChromePath      *string            `yaml:"custom_chrome_path"`
	Width                 uint64             `yaml:"width"`
	Height                uint64             `yaml:"height"`
	XvfbDpi               uint64             `yaml:"xvfb_dpi"`
	CopyToPath            CopyToPathSettings `yaml:"copy_to_path"`
	PostProcessingScripts []string           `yaml:"post_processing_scripts"`
}

type CopyToPathSettings struct {
	MainPath string `yaml:"main_path"`
	SubPath  string `yaml:"sub_path"`
}

type LogSettings struct {
	LogFile    string  `yaml:"log_file"`
	MaxSize    int     `yaml:"max_size"`
	MaxBackups int     `yaml:"max_backups"`
	MaxAge     int     `yaml:"max_age"`
	LogLevel   *string `yaml:"log_level"`
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
}

var appCnf *AppConfig

func New(a *AppConfig) {
	if appCnf != nil {
		// not allow multiple config
		return
	}

	appCnf = new(AppConfig) // otherwise will give error
	// now set the config
	appCnf = a

	appCnf.setLogger()
	appCnf.setDefaultConfig()
}

func (a *AppConfig) setDefaultConfig() {
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

	if a.FfmpegSettings == nil {
		commonPostInput := "-c:v libx264 -x264-params keyint=120:scenecut=0 -preset veryfast -crf 23 -c:a aac -af highpass=f=200,lowpass=f=2000,afftdn -async 1 -movflags frag_keyframe+empty_moov+default_base_moof -flush_packets 1 -tune zerolatency"

		a.FfmpegSettings = &FfmpegSettings{
			Recording: FfmpegOptions{
				PreInput:  "-loglevel error -thread_queue_size 512 -draw_mouse 0",
				PostInput: fmt.Sprintf("%s -y", commonPostInput),
			},
			PostRecording: FfmpegOptions{
				PreInput:  "-loglevel error",
				PostInput: "-preset veryfast -movflags faststart -y",
			},
			Rtmp: FfmpegOptions{
				PreInput:  "-loglevel error -draw_mouse 0",
				PostInput: fmt.Sprintf("%s -pix_fmt yuv420p -b:v 2500k -video_size 1920x1080 -b:a 128k -ar 44100 -bufsize 5000k -f flv", commonPostInput),
			},
		}
	}
}

func (a *AppConfig) setLogger() {
	p := a.LogSettings.LogFile
	if strings.HasPrefix(p, "./") {
		p = filepath.Join(a.RootWorkingDir, p)
	}

	logLevel := logrus.WarnLevel
	if a.LogSettings.LogLevel != nil && *a.LogSettings.LogLevel != "" {
		if lv, err := logrus.ParseLevel(strings.ToLower(*a.LogSettings.LogLevel)); err == nil {
			logLevel = lv
		}
	}

	logWriter := &lumberjack.Logger{
		Filename:   p,
		MaxSize:    a.LogSettings.MaxSize,
		MaxBackups: a.LogSettings.MaxBackups,
		MaxAge:     a.LogSettings.MaxAge,
	}

	logrus.SetLevel(logLevel)
	logrus.SetReportCaller(true)
	logrus.SetFormatter(&logrus.JSONFormatter{
		PrettyPrint:       true,
		DisableHTMLEscape: true,
	})
	logrus.RegisterExitHandler(func() {
		_ = logWriter.Close()
	})

	var w io.Writer
	if a.Recorder.Debug {
		w = io.MultiWriter(os.Stdout, logWriter)
	} else {
		w = io.Writer(logWriter)
	}
	logrus.SetOutput(w)
}

func GetConfig() *AppConfig {
	return appCnf
}

func GetLogger() *logrus.Logger {
	return logrus.StandardLogger()
}
