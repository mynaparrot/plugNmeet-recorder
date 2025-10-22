package recorder

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/sirupsen/logrus"
)

// creates a new xvfb display
func (r *Recorder) launchXvfb() error {
	r.displayId = fmt.Sprintf(":%d%d", r.Req.RoomTableId, r.Req.Task)
	log := r.Logger.WithField("displayId", r.displayId)

	args := []string{
		r.displayId,
		"-nocursor",
		"-screen", "0", fmt.Sprintf("%dx%dx24", r.AppCnf.Recorder.Width, r.AppCnf.Recorder.Height),
		"-ac",
		"-nolisten", "tcp",
		"-nolisten", "unix",
		"-dpi", fmt.Sprintf("%d", r.AppCnf.Recorder.XvfbDpi),
		"+extension", "RANDR",
	}
	log.WithField("args", args).Infof("creating X display")

	xvfb := exec.CommandContext(r.ctx, "Xvfb", args...)
	xvfb.Stderr = &infoLogger{cmd: "xvfb", logger: log}
	if err := xvfb.Start(); err != nil {
		return fmt.Errorf("xvfb failed to start: %w", err)
	}
	r.Lock()
	r.xvfbCmd = xvfb
	r.Unlock()

	go func() {
		err := r.xvfbCmd.Wait()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				// Don't log expected exit codes during a graceful shutdown.
				if exitErr.ExitCode() != -1 && exitErr.ExitCode() != 255 {
					log.Errorf("xvfb exited with unexpected code: %d", exitErr.ExitCode())
				}
			}
			r.Close(plugnmeet.RecordingTasks_STOP, err)
		}
	}()
	return nil
}

func (r *Recorder) closeXvfb(log *logrus.Entry) {
	r.Lock()
	defer r.Unlock()

	if r.xvfbCmd != nil {
		log.Infoln("closing X display")

		if err := r.xvfbCmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			log.Errorf("failed to interrupt X display: %v, trying to kill", err)
			_ = r.xvfbCmd.Process.Kill()
		}
		r.xvfbCmd = nil
	}
}
