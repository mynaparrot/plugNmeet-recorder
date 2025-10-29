package recorder

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/sirupsen/logrus"
	"mvdan.cc/sh/v3/shell"
)

func (r *Recorder) launchFfmpegProcess() error {
	var args []string
	var preInput, postInput string

	if r.Req.Task == plugnmeet.RecordingTasks_START_RTMP {
		preInput = r.AppCnf.FfmpegSettings.Rtmp.PreInput
		postInput = r.AppCnf.FfmpegSettings.Rtmp.PostInput
	} else if r.Req.Task == plugnmeet.RecordingTasks_START_RECORDING {
		preInput = r.AppCnf.FfmpegSettings.Recording.PreInput
		postInput = r.AppCnf.FfmpegSettings.Recording.PostInput
	} else {
		return fmt.Errorf("invalid task %s received", r.Req.Task.String())
	}

	preArgs, err := shell.Fields(preInput, nil)
	if err != nil {
		return fmt.Errorf("failed to parse ffmpeg pre-input args: %w", err)
	}
	args = append(args, preArgs...)

	args = append(args,
		"-video_size", fmt.Sprintf("%dx%d", r.AppCnf.Recorder.Width, r.AppCnf.Recorder.Height),
		"-f", "x11grab",
		"-i", r.displayId,
		"-f", "pulse",
		"-i", fmt.Sprintf("%s.monitor", r.pulseSinkName),
	)

	postArgs, err := shell.Fields(postInput, nil)
	if err != nil {
		return fmt.Errorf("failed to parse ffmpeg post-input args: %w", err)
	}
	args = append(args, postArgs...)

	if r.Req.Task == plugnmeet.RecordingTasks_START_RTMP {
		args = append(args, *r.Req.RtmpUrl)
	} else {
		args = append(args, r.recordingFilePath)
	}
	log := r.Logger.WithField("args", args)
	log.Infof("starting ffmpeg process")

	ffmpegCmd := exec.CommandContext(r.ctx, "ffmpeg", args...)
	// Pass the contextual logger to the infoLogger for stderr
	ffmpegCmd.Stderr = &infoLogger{cmd: "ffmpeg", logger: log}
	if err := ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg failed to start: %w", err)
	}
	r.Lock()
	r.ffmpegCmd = ffmpegCmd
	r.Unlock()

	go func() {
		err := r.ffmpegCmd.Wait()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				// Don't log expected exit codes during shutdown.
				if exitErr.ExitCode() != -1 && exitErr.ExitCode() != 255 {
					log.Errorf("ffmpeg exited with unexpected code: %d", exitErr.ExitCode())
				}
			}
			r.Close(plugnmeet.RecordingTasks_STOP, err)
		}
	}()

	// so, if everything goes well then we can make callback
	if r.OnAfterStartCallback != nil {
		// Pass the logger to the callback, as per the new signature.
		r.OnAfterStartCallback(r.Req, r.Logger)
	}
	return nil
}

func (r *Recorder) closeFfmpeg(log *logrus.Entry) {
	r.Lock()
	defer r.Unlock()

	if r.ffmpegCmd != nil {
		log.Infoln("closing ffmpeg")

		if err := r.ffmpegCmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			log.Errorf("failed to interrupt ffmpeg: %v, trying to kill", err)
			_ = r.ffmpegCmd.Process.Kill()
		}
		r.ffmpegCmd = nil
	}
}
