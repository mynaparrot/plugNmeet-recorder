#!/usr/bin/env bash

set -euxo pipefail

# cleanup
rm -rf /tmp/.X* /var/run/pulse /var/lib/pulse /root/.config/pulse /root/.cache/xdgr/pulse
# start pulseaudio
pulseaudio -D --verbose --exit-idle-time=-1 --disallow-exit

# we'll navigate to /app
# this will ensure to read config.yaml or write logs in correct dir
mkdir -p /app
cd /app
# Run recorder service
exec plugnmeet-recorder "$@"
