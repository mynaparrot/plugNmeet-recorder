#!/usr/bin/env bash

set -euxo pipefail

# cleanup
# NOTE: clean the actual pulseaudio runtime dir ($XDG_RUNTIME_DIR/pulse). The image
# sets XDG_RUNTIME_DIR=/home/root/.cache/xdgr, but this previously removed
# /root/.cache/xdgr/pulse, so a stale pid/socket survived a restart and made the next
# `pulseaudio -D` fail ("Daemon startup failed") -> crash-loop under `set -e`.
rm -rf /tmp/.X* /var/run/pulse /var/lib/pulse /root/.config/pulse "${XDG_RUNTIME_DIR:?}/pulse"
# start pulseaudio
pulseaudio -D --verbose --exit-idle-time=-1 --disallow-exit

# we'll navigate to /app
# this will ensure to read config.yaml or write logs in correct dir
mkdir -p /app
cd /app
# Run recorder service
exec plugnmeet-recorder "$@"
