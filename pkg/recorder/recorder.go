package recorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	log "github.com/sirupsen/logrus"
)

const (
	waitForSelectorTimeout = time.Second * 30
	shutdownTimeout        = time.Second * 5
)

type Recorder struct {
	joinUrl  string
	filePath string
	fileName string

	Req                  *plugnmeet.PlugNmeetToRecorder
	AppCnf               *config.AppConfig
	OnAfterStartCallback func(req *plugnmeet.PlugNmeetToRecorder)
	OnAfterCloseCallback func(req *plugnmeet.PlugNmeetToRecorder, filePath, fileName string, err error)

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

func New(r *Recorder) *Recorder {
	r.ctx, r.ctxCancel = context.WithCancel(context.Background())
	return r
}

func (r *Recorder) Start() error {
	var err error
	defer func() {
		if err != nil {
			log.Errorln(fmt.Sprintf("failed to start recorder for task: %s, roomTableId: %d, error: %v", r.Req.Task.String(), r.Req.GetRoomTableId(), err))
			r.Close(err)
		}
	}()

	if r.Req.Task == plugnmeet.RecordingTasks_START_RECORDING {
		r.filePath = path.Join(r.AppCnf.Recorder.CopyToPath.MainPath, r.AppCnf.Recorder.CopyToPath.SubPath, r.Req.GetRoomId())
		err = os.MkdirAll(r.filePath, 0755)
		if err != nil {
			return err
		}
		r.fileName = r.Req.GetRecordingId() + "_raw.mp4"
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

func (r *Recorder) Close(err error) {
	r.closeOnce.Do(func() {
		// timeout for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(r.ctx, shutdownTimeout)
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			r.closeFfmpeg()
			r.closeChromeDp()
			r.closeXvfb()
			r.closePulse(shutdownCtx)
		}()

		select {
		case <-done:
			log.Infoln("graceful shutdown finished for task:", r.Req.Task.String())
		case <-shutdownCtx.Done():
			log.Errorln("graceful shutdown timed out for task:", r.Req.Task.String(), "forcing close")
		}

		if r.OnAfterCloseCallback != nil {
			r.OnAfterCloseCallback(r.Req, r.filePath, r.fileName, err)
		}

		// close everything if still running
		r.ctxCancel()
	})
}

type infoLogger struct {
	cmd string
}

func (l *infoLogger) Write(p []byte) (int, error) {
	log.Infoln(fmt.Sprintf("%s: %s", l.cmd, string(p)))
	return len(p), nil
}
