package recorder

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// creates a new xvfb display
func (r *Recorder) launchXvfb() error {
	r.displayId = fmt.Sprintf(":%d%d", r.Req.RoomTableId, r.Req.Task)

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
	r.Logger.Infof("creating X display with args: %s", strings.Join(args, " "))

	xvfb := exec.CommandContext(r.ctx, "Xvfb", args...)
	xvfb.Stderr = &infoLogger{cmd: "xvfb", logger: r.Logger}
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
					r.Logger.Errorf("xvfb exited with unexpected code: %d", exitErr.ExitCode())
				}
			}
			r.Close(err)
		}
	}()
	return nil
}

func (r *Recorder) closeXvfb() {
	r.Lock()
	defer r.Unlock()

	if r.xvfbCmd != nil {
		r.Logger.Infoln("closing X display")

		if err := r.xvfbCmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			r.Logger.Errorf("failed to interrupt X display: %v, trying to kill", err)
			_ = r.xvfbCmd.Process.Kill()
		}
		r.xvfbCmd = nil
	}
}
