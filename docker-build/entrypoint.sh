#!/usr/bin/env bash

set -euxo pipefail

# cleanup
# NOTE: also clean the actual pulseaudio runtime dir at /home/root/.cache/xdgr/pulse
# (the image sets XDG_RUNTIME_DIR=/home/root/.cache/xdgr). Previously only
# /root/.cache/xdgr/pulse was removed, so a stale pid/socket survived a restart and made
# the next `pulseaudio -D` fail ("Daemon startup failed") -> crash-loop under `set -e`.
# Additive: keeps the existing path, adds the real one; no new behavior, no env required.
rm -rf /tmp/.X* /var/run/pulse /var/lib/pulse /root/.config/pulse /root/.cache/xdgr/pulse /home/root/.cache/xdgr/pulse
# start pulseaudio
pulseaudio -D --verbose --exit-idle-time=-1 --disallow-exit

# we'll navigate to /app
# this will ensure to read config.yaml or write logs in correct dir
mkdir -p /app
cd /app
# Run recorder service
exec plugnmeet-recorder "$@"
