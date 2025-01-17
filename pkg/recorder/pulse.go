package recorder

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os/exec"
	"strings"
)

func (r *Recorder) createPulseSink() error {
	r.pulseSinkName = fmt.Sprintf("%d-%d", r.Req.RoomTableId, r.Req.Task)
	log.Infoln(fmt.Sprintf("Create pulse sink %s for task: %s", r.pulseSinkName, r.Req.Task))

	cmd := exec.CommandContext(r.ctx, "pactl",
		"load-module",
		"module-null-sink",
		fmt.Sprintf("sink_name=\"%s\"", r.pulseSinkName),
		fmt.Sprintf("sink_properties=device.description=\"%s\"", r.pulseSinkName),
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New("pulse " + err.Error())
	}

	r.Lock()
	r.pulseSinkId = strings.TrimSpace(string(b))
	r.Unlock()
	return nil
}

func (r *Recorder) closePulse() {
	if r.pulseSinkId != "" {
		log.Infoln("unloading pulse module:", r.pulseSinkId)
		cmd := exec.CommandContext(r.ctx, "pactl", "unload-module", r.pulseSinkId)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Errorln("failed to unload pulse sink", err)
		}
	}
}
