#!/usr/bin/env bash
set -euo pipefail

HOST="root@mediaserver"
BIN_DST="/srv/comfychan/comfychan"
DATA_DIR="/var/lib/comfychan"
SERVICE="comfychan"

echo "==> Building"
templ generate
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ./out/comfychan ./web

echo "==> Ensuring remote directories exist"
ssh "$HOST" "mkdir -p $DATA_DIR/web/static $DATA_DIR/internal/database"

echo "==> Uploading binary"
# Upload beside the live binary, then atomically rename over it.
scp ./out/comfychan "$HOST:${BIN_DST}.new"
ssh "$HOST" "mv ${BIN_DST}.new $BIN_DST"

echo "==> Uploading static assets"
scp -r web/static/. "$HOST:$DATA_DIR/web/static/"

echo "==> Restarting $SERVICE"
ssh "$HOST" "systemctl restart $SERVICE && systemctl --no-pager status $SERVICE"

echo "==> Done"
