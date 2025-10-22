package recorder

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

func (r *Recorder) createPulseSink() error {
	r.pulseSinkName = fmt.Sprintf("%d-%d", r.Req.RoomTableId, r.Req.Task)
	log := r.Logger.WithField("pulseSinkName", r.pulseSinkName)

	args := []string{
		"load-module",
		"module-null-sink",
		fmt.Sprintf("sink_name=\"%s\"", r.pulseSinkName),
		fmt.Sprintf("sink_properties=device.description=\"%s\"", r.pulseSinkName),
	}
	log.WithField("args", args).Infof("creating pulse sink")

	cmd := exec.CommandContext(r.ctx, "pactl", args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pulse: %w", err)
	}

	r.Lock()
	r.pulseSinkId = strings.TrimSpace(string(b))
	log.WithField("pulseSinkId", r.pulseSinkId).Infof("pulse sink created successfully")
	r.Unlock()

	return nil
}

func (r *Recorder) closePulse(log *logrus.Entry, ctx context.Context) {
	r.Lock()
	defer r.Unlock()

	if r.pulseSinkId != "" {
		log.Infof("unloading pulse module: %s", r.pulseSinkId)

		cmd := exec.CommandContext(ctx, "pactl", "unload-module", r.pulseSinkId)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Errorf("failed to unload pulse sink: %v", err)
		}
		r.pulseSinkId = ""
	}
}
