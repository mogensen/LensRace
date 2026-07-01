#!/bin/sh
# Runs as root so it can fix up ownership of a freshly-mounted persistent
# disk (Render — and other platforms — mount volumes root-owned regardless
# of what the image had at that path at build time), then drops to the
# unprivileged app user before exec'ing the server.
set -e

db_dir=$(dirname "${DB_PATH:-/data/lensrace.db}")
mkdir -p "$db_dir"
chown app:app "$db_dir"

exec su-exec app ./server
