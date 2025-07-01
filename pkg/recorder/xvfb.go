package recorder

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
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
	log.Infoln(fmt.Sprintf("creating X dispaly for task: %s with agrs: %s", r.Req.Task, strings.Join(args, " ")))

	xvfb := exec.CommandContext(r.ctx, "Xvfb", args...)
	xvfb.Stderr = &infoLogger{cmd: "xvfb"}
	if err := xvfb.Start(); err != nil {
		return errors.New("xvfb: " + err.Error())
	}
	r.Lock()
	r.xvfbCmd = xvfb
	r.Unlock()

	go func() {
		err := r.xvfbCmd.Wait()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				log.Errorln(fmt.Errorf("xvfb exited with code: %d for task: %s, roomTableId: %d", exitErr.ExitCode(), r.Req.Task.String(), r.Req.GetRoomTableId()))
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
		log.Infoln(fmt.Sprintf("closing X display for task: %s, roomTableId: %d", r.Req.Task.String(), r.Req.GetRoomTableId()))

		if err := r.xvfbCmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			log.Errorln("failed to interrupt X display:", err.Error(), "so, trying to kill")
			_ = r.xvfbCmd.Process.Kill()
		}
		r.xvfbCmd = nil
	}
}
