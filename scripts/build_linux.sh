#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# 自动查找 Go 二进制
GO=$(which go 2>/dev/null || true)
if [ -z "$GO" ]; then
    for d in /usr/local/go/bin /usr/lib/go/bin /snap/go/bin "$HOME/go/bin" "$HOME/sdk/go/bin"; do
        if [ -x "$d/go" ]; then GO="$d/go"; break; fi
    done
fi
if [ -z "$GO" ]; then
    echo "找不到 go，请确认 Go 已安装并在以上路径中" >&2
    exit 1
fi

# 构建缓存放在 /tmp（避免系统分区只读）
CACHE_DIR="/tmp/dns-switch-build"
mkdir -p "$CACHE_DIR/go-build" "$CACHE_DIR/gomod" "$CACHE_DIR/gopath"

export GOCACHE="$CACHE_DIR/go-build"
export GOMODCACHE="$CACHE_DIR/gomod"
export GOPATH="$CACHE_DIR/gopath"

"$GO" build -ldflags "-s -w" -o dns-switch .
echo "构建完成: dns-switch"
