package recorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	"github.com/sirupsen/logrus"
)

const (
	waitForSelectorTimeout = time.Second * 30
	shutdownTimeout        = time.Second * 5
)

type Recorder struct {
	joinUrl  string
	filePath string
	// fileName will be the raw file, e.g. recordingId_raw.mkv
	fileName string

	Req                  *plugnmeet.PlugNmeetToRecorder
	AppCnf               *config.AppConfig
	Logger               *logrus.Entry
	OnAfterStartCallback func(req *plugnmeet.PlugNmeetToRecorder, logger *logrus.Entry)
	OnAfterCloseCallback func(req *plugnmeet.PlugNmeetToRecorder, filePath, fileName string, err error, logger *logrus.Entry)

	ctx           context.Context
	ctxCancel     context.CancelFunc
	displayId     string
	pulseSinkName string
	pulseSinkId   string
	xvfbCmd       *exec.Cmd
	ffmpegCmd     *exec.Cmd
	closeChrome   context.CancelFunc

	sync.Mutex
	closeOnce sync.Once
}

func New(mainCtx context.Context, r *Recorder) *Recorder {
	r.ctx, r.ctxCancel = context.WithCancel(mainCtx)
	return r
}

func (r *Recorder) Start() error {
	var err error
	defer func() {
		if err != nil {
			r.Logger.Errorf("failed to start recorder: %v", err)
			r.Close(plugnmeet.RecordingTasks_STOP, err)
		}
	}()

	if r.Req.Task == plugnmeet.RecordingTasks_START_RECORDING {
		r.filePath = path.Join(r.AppCnf.Recorder.CopyToPath.MainPath, r.AppCnf.Recorder.CopyToPath.SubPath, r.Req.GetRoomId())
		err = os.MkdirAll(r.filePath, 0755)
		if err != nil {
			return err
		}

		// For backward compatibility, we'll check which codec is in use.
		// ffv1 must be stored in .mkv
		if strings.Contains(r.AppCnf.FfmpegSettings.Recording.PostInput, "ffv1") {
			r.fileName = r.Req.GetRecordingId() + "_raw.mkv"
		} else {
			r.fileName = r.Req.GetRecordingId() + "_raw.mp4"
		}
	}

	r.joinUrl = fmt.Sprintf("%s/?access_token=%s", r.AppCnf.PlugNmeetInfo.Host, r.Req.GetAccessToken())
	if r.AppCnf.PlugNmeetInfo.JoinHost != nil && *r.AppCnf.PlugNmeetInfo.JoinHost != "" {
		r.joinUrl = *r.AppCnf.PlugNmeetInfo.JoinHost + r.Req.GetAccessToken()
	}

	if err = r.createPulseSink(); err != nil {
		return err
	}
	if err = r.launchXvfb(); err != nil {
		return err
	}

	// start chrome in go routine and return response immediately
	// otherwise user will see error message if wait too long
	// if something goes wrong then we can know from callback
	go r.launchChrome()
	return nil
}

func (r *Recorder) Close(task plugnmeet.RecordingTasks, err error) {
	r.closeOnce.Do(func() {
		log := r.Logger.WithField("closeTask", task)
		log.Infoln("starting to close recorder")

		// timeout for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(r.ctx, shutdownTimeout)
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			r.closeFfmpeg(log)
			r.closeChromeDp(log)
			r.closeXvfb(log)
			r.closePulse(log, shutdownCtx)
		}()

		select {
		case <-done:
			log.Infoln("graceful shutdown finished")
		case <-shutdownCtx.Done():
			log.Errorln("graceful shutdown timed out, forcing close")
		}

		if r.OnAfterCloseCallback != nil {
			r.OnAfterCloseCallback(r.Req, r.filePath, r.fileName, err, log)
		}

		// close everything if still running
		r.ctxCancel()
	})
}

// infoLogger is used to capture stdout/stderr of external commands.
type infoLogger struct {
	cmd    string
	logger *logrus.Entry
}

func (l *infoLogger) Write(p []byte) (int, error) {
	l.logger.Infof("%s: %s", l.cmd, p)
	return len(p), nil
}
