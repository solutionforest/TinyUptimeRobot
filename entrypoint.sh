#!/bin/sh
# Fix ownership of the data dir (bind mounts are often owned by root),
# then drop privileges and run as the unprivileged monitor user.
set -e

DATA_DIR="$(dirname "${LOG_FILE:-/app/data/status.txt}")"
mkdir -p "$DATA_DIR" 2>/dev/null || true
chown -R monitor:monitor "$DATA_DIR" 2>/dev/null || true

# if the targets file is read-only and unreadable, that's fine — the app will error clearly
exec su-exec monitor:monitor /app/TinyUptimeRobot "$@"
