#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# Try to locate the Go binary — it may not be in PATH
go_cmd=""
for candidate in \
	"$(command -v go 2>/dev/null || true)" \
	/usr/local/go/bin/go \
	/usr/lib/go/bin/go \
	/snap/bin/go \
	"$HOME/go/bin/go" \
	"$HOME/.go/bin/go" \
	"$HOME/sdk/go"*"/bin/go"; do
	if [ -x "$candidate" ]; then
		go_cmd="$candidate"
		break
	fi
done

if [ -z "$go_cmd" ]; then
	echo "ERR: 未找到 Go 编译器。请安装 Go 1.22+ (https://go.dev/dl/)" >&2
	echo "     安装后执行: go build -o dns-switch ." >&2
	exit 1
fi

# 构建缓存放 /tmp（避免系统分区只读）
CACHE_DIR="/tmp/dns-switch-build"
mkdir -p "$CACHE_DIR/go-build" "$CACHE_DIR/gomod" "$CACHE_DIR/gopath"
export GOCACHE="$CACHE_DIR/go-build"
export GOMODCACHE="$CACHE_DIR/gomod"
export GOPATH="$CACHE_DIR/gopath"

"$go_cmd" build -o dns-switch .
exec ./dns-switch "$@"
