package recorder

import (
	"context"
	"errors"
	"fmt"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/mynaparrot/plugnmeet-recorder/pkg/config"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"
)

const (
	waitForSelectorTimeout = time.Second * 30
)

type Recorder struct {
	joinUrl  string
	filePath string
	fileName string
	isClosed bool

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
}

func New(r *Recorder) *Recorder {
	r.ctx, r.ctxCancel = context.WithCancel(context.Background())
	return r
}

func (r *Recorder) Start() error {
	if r.Req.Task == plugnmeet.RecordingTasks_START_RECORDING {
		r.filePath = path.Join(r.AppCnf.Recorder.CopyToPath.MainPath, r.AppCnf.Recorder.CopyToPath.SubPath, r.Req.GetRoomSid())
		err := os.MkdirAll(r.filePath, 0755)
		if err != nil {
			switch {
			case errors.Is(err, os.ErrExist):
				log.Infoln(fmt.Sprintf("%s already exists", r.filePath))
			default:
				return err
			}
		}

		r.fileName = r.Req.GetRecordingId() + "_raw.mp4"
	}

	r.joinUrl = r.AppCnf.PlugNmeetInfo.Host + "/?access_token=" + r.Req.GetAccessToken()
	if r.AppCnf.PlugNmeetInfo.JoinHost != nil && *r.AppCnf.PlugNmeetInfo.JoinHost != "" {
		r.joinUrl = *r.AppCnf.PlugNmeetInfo.JoinHost + r.Req.GetAccessToken()
	}

	err := r.createPulseSink()
	if err != nil {
		r.Close(err)
		return err
	}
	err = r.launchXvfb()
	if err != nil {
		r.Close(err)
		return err
	}

	go r.launchChrome()
	return nil
}

func (r *Recorder) Close(err error) {
	r.Lock()
	defer r.Unlock()

	if !r.isClosed {
		time.Sleep(time.Second * 1)

		r.closeFfmpeg()
		r.closeChromeDp()
		r.closeXvfb()
		r.closePulse()

		if r.OnAfterCloseCallback != nil {
			r.OnAfterCloseCallback(r.Req, r.filePath, r.fileName, err)
		}

		time.Sleep(time.Second * 1)
		// close everything
		r.ctxCancel()
		r.isClosed = true
	}
}

type infoLogger struct {
	cmd string
}

func (l *infoLogger) Write(p []byte) (int, error) {
	log.Infoln(fmt.Sprintf("%s: %s", l.cmd, string(p)))
	return len(p), nil
}
