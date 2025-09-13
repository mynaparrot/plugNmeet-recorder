package recorder

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func (r *Recorder) createPulseSink() error {
	r.pulseSinkName = fmt.Sprintf("%d-%d", r.Req.RoomTableId, r.Req.Task)

	args := []string{
		"load-module",
		"module-null-sink",
		fmt.Sprintf("sink_name=\"%s\"", r.pulseSinkName),
		fmt.Sprintf("sink_properties=device.description=\"%s\"", r.pulseSinkName),
	}
	r.Logger.Infof("creating pulse sink with args: %s", strings.Join(args, " "))

	cmd := exec.CommandContext(r.ctx, "pactl", args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pulse: %w", err)
	}

	r.Lock()
	r.pulseSinkId = strings.TrimSpace(string(b))
	r.Logger.Infof("pulse sink created successfully with id: %s", r.pulseSinkId)
	r.Unlock()

	return nil
}

func (r *Recorder) closePulse(ctx context.Context) {
	r.Lock()
	defer r.Unlock()

	if r.pulseSinkId != "" {
		r.Logger.Infof("unloading pulse module: %s", r.pulseSinkId)

		cmd := exec.CommandContext(ctx, "pactl", "unload-module", r.pulseSinkId)
		if _, err := cmd.CombinedOutput(); err != nil {
			r.Logger.Errorf("failed to unload pulse sink: %v", err)
		}
		r.pulseSinkId = ""
	}
}
