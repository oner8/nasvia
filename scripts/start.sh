#!/usr/bin/env bash
# NASVIA 裸机启动脚本：构建产物检查 + 环境变量默认值 + 启动服务
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

BIN_PATH="${NASVIA_BIN:-${ROOT_DIR}/bin/nasvia}"
DATA_DIR="${NASVIA_DATA_DIR:-${ROOT_DIR}/data}"
PORT="${NASVIA_PORT:-3720}"

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "未找到可执行文件：${BIN_PATH}" >&2
  echo "请先构建：cd ${ROOT_DIR}/web && npm ci && npm run build" >&2
  echo "然后：CGO_ENABLED=0 go build -trimpath -ldflags=\"-s -w\" -o bin/nasvia ./cmd/nasvia" >&2
  exit 1
fi

mkdir -p "${DATA_DIR}"

if [[ -z "${NASVIA_PASSWORD:-}" ]]; then
  echo "提示：未设置 NASVIA_PASSWORD，后台管理接口将拒绝访问（仅可浏览公开内容）。"
fi

export NASVIA_PORT="${PORT}"
export NASVIA_DATA_DIR="${DATA_DIR}"

echo "启动 NASVIA：端口 ${PORT}，数据目录 ${DATA_DIR}"
exec "${BIN_PATH}" "$@"
