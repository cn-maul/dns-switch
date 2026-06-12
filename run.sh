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

"$go_cmd" build -o dns-switch .
exec ./dns-switch "$@"
