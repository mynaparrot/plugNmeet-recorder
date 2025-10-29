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
	joinUrl           string
	recordingFilePath string // Full path to the raw file being written (can be temp or final)
	finalRawFilePath  string // Full path to the final destination of the raw file

	Req                  *plugnmeet.PlugNmeetToRecorder
	AppCnf               *config.AppConfig
	Logger               *logrus.Entry
	OnAfterStartCallback func(req *plugnmeet.PlugNmeetToRecorder, logger *logrus.Entry)
	OnAfterCloseCallback func(req *plugnmeet.PlugNmeetToRecorder, recordingFilePath, finalRawFilePath string, err error, logger *logrus.Entry)

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
		var fileName string
		if strings.Contains(r.AppCnf.FfmpegSettings.Recording.PostInput, "ffv1") {
			fileName = r.Req.GetRecordingId() + "_raw.mkv"
		} else {
			fileName = r.Req.GetRecordingId() + "_raw.mp4"
		}

		// Final destination path on the network drive
		finalPath := path.Join(r.AppCnf.Recorder.CopyToPath.MainPath, r.AppCnf.Recorder.CopyToPath.SubPath, r.Req.GetRoomId())
		r.finalRawFilePath = path.Join(finalPath, fileName)

		// Determine the recording path (temporary or final)
		recordPath := finalPath
		if r.AppCnf.Recorder.TemporaryDir != nil && *r.AppCnf.Recorder.TemporaryDir != "" {
			recordPath = path.Join(*r.AppCnf.Recorder.TemporaryDir, r.Req.GetRoomId())
			r.Logger.Infof("using temporary recording path: %s", recordPath)
		}
		r.recordingFilePath = path.Join(recordPath, fileName)

		// Create the directory where the file will be written
		err = os.MkdirAll(recordPath, 0755)
		if err != nil {
			return err
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
			r.OnAfterCloseCallback(r.Req, r.recordingFilePath, r.finalRawFilePath, err, log)
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
