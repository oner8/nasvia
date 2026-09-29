#!/bin/sh
# NASVIA 容器入口：以 root 启动时先把数据目录属主修正为 PUID:PGID，再降权运行服务。
# 这样绑定宿主目录（如群晖 /volume1/docker/nasvia-data:/data）也不必手动 chmod。
set -eu

BIN=/usr/local/bin/nasvia

# 已用 user: / --user 指定非 root 身份运行：无权 chown，直接启动（目录权限需自行保证）。
if [ "$(id -u)" != "0" ]; then
  exec "$BIN" "$@"
fi

PUID="${PUID:-10001}"
PGID="${PGID:-10001}"
DATA_DIR="${NASVIA_DATA_DIR:-/data}"

mkdir -p "$DATA_DIR"
# 只在存在属主不符的文件时才递归 chown，避免每次启动都全量扫描改写。
if [ -n "$(find "$DATA_DIR" \( ! -user "$PUID" -o ! -group "$PGID" \) -print 2>/dev/null | head -n 1)" ]; then
  echo "[nasvia] 修正数据目录属主：$DATA_DIR -> $PUID:$PGID"
  chown -R "$PUID:$PGID" "$DATA_DIR"
fi

exec su-exec "$PUID:$PGID" "$BIN" "$@"
