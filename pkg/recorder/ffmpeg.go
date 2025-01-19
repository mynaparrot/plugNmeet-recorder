package recorder

import (
	"errors"
	"fmt"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
)

func (r *Recorder) launchFfmpegProcess(mp4File string) error {
	var args []string

	if r.Req.Task == plugnmeet.RecordingTasks_START_RTMP {
		args = append(args, strings.Split(r.AppCnf.FfmpegSettings.Rtmp.PreInput, " ")...)
		args = append(args,
			"-video_size", fmt.Sprintf("%dx%d", r.AppCnf.Recorder.Width, r.AppCnf.Recorder.Height),
			"-f", "x11grab",
			"-i", r.displayId,
			"-f", "pulse",
			"-i", fmt.Sprintf("%s.monitor", r.pulseSinkName),
		)
		args = append(args, strings.Split(r.AppCnf.FfmpegSettings.Rtmp.PostInput, " ")...)
		args = append(args, *r.Req.RtmpUrl)
	} else if r.Req.Task == plugnmeet.RecordingTasks_START_RECORDING {
		args = append(args, strings.Split(r.AppCnf.FfmpegSettings.Recording.PreInput, " ")...)
		args = append(args,
			"-video_size", fmt.Sprintf("%dx%d", r.AppCnf.Recorder.Width, r.AppCnf.Recorder.Height),
			"-f", "x11grab",
			"-i", r.displayId,
			"-f", "pulse",
			"-i", fmt.Sprintf("%s.monitor", r.pulseSinkName),
		)
		args = append(args, strings.Split(r.AppCnf.FfmpegSettings.Recording.PostInput, " ")...)
		args = append(args, mp4File)
	} else {
		return errors.New(fmt.Sprintf("invalid task %s received", r.Req.Task.String()))
	}
	log.Infoln(fmt.Sprintf("starting ffmpeg process for Task: %s with args: %s", r.Req.Task.String(), strings.Join(args, " ")))

	ffmpegCmd := exec.CommandContext(r.ctx, "ffmpeg", args...)
	ffmpegCmd.Stderr = &infoLogger{cmd: "ffmpeg"}
	if err := ffmpegCmd.Start(); err != nil {
		return errors.New("ffmpeg: " + err.Error())
	}
	r.Lock()
	r.ffmpegCmd = ffmpegCmd
	r.Unlock()

	go func() {
		err := r.ffmpegCmd.Wait()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				if exitErr.ExitCode() != -1 && exitErr.ExitCode() != 255 {
					log.Errorln(fmt.Errorf("ffmpeg exited with code: %d for task: %s, roomTableId: %d", exitErr.ExitCode(), r.Req.Task.String(), r.Req.GetRoomTableId()))
				}
			}
			r.Close(err)
		}
	}()

	// so, if everything goes well then we can make callback
	r.Lock()
	if r.OnAfterStartCallback != nil {
		r.OnAfterStartCallback(r.Req)
	}
	r.Unlock()
	return nil
}

func (r *Recorder) closeFfmpeg() {
	if r.ffmpegCmd != nil {
		log.Infoln(fmt.Sprintf("closing ffmpeg for task: %s, roomTableId: %d", r.Req.Task.String(), r.Req.GetRoomTableId()))
		if err := r.ffmpegCmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			log.Errorln("failed to interrupt ffmpeg:", err.Error(), "so, trying to kill")
			_ = r.ffmpegCmd.Process.Kill()
		}
		r.ffmpegCmd = nil
	}
}
