#!/bin/sh
# Fix ownership AND permissions of the data dir (bind mounts are often
# owned by root or lose mode bits), then drop privileges and run as the
# unprivileged monitor user.
set -e

DATA_DIR="$(dirname "${LOG_FILE:-/app/data/status.txt}")"
mkdir -p "$DATA_DIR" 2>/dev/null || true
chown -R monitor:monitor "$DATA_DIR" 2>/dev/null || true
# ensure the owner can traverse/write the dir (x bit is required to open files inside)
chmod u+rwx "$DATA_DIR" 2>/dev/null || true

exec su-exec monitor:monitor /app/TinyUptimeRobot "$@"
