package recorder

import (
	"context"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
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
	log.Infoln(fmt.Sprintf("creating pulse sink for task: %s with agrs: %s", r.Req.Task, strings.Join(args, " ")))

	cmd := exec.CommandContext(r.ctx, "pactl", args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New("pulse: " + err.Error())
	}

	r.Lock()
	r.pulseSinkId = strings.TrimSpace(string(b))
	log.Infoln("pulse sink created successfully with id:", r.pulseSinkId)
	r.Unlock()

	return nil
}

func (r *Recorder) closePulse(ctx context.Context) {
	r.Lock()
	defer r.Unlock()

	if r.pulseSinkId != "" {
		log.Infoln(fmt.Sprintf("unloading pulse module: %s for task: %s, roomTableId: %d", r.pulseSinkId, r.Req.Task.String(), r.Req.GetRoomTableId()))

		cmd := exec.CommandContext(ctx, "pactl", "unload-module", r.pulseSinkId)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Errorln("failed to unload pulse sink", err)
		}
		r.pulseSinkId = ""
	}
}
