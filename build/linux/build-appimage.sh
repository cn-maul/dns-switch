#!/bin/bash
# Build DNS-Switch as an AppImage with self-contained GTK4/WebKitGTK.
# Usage: ./build/linux/build-appimage.sh
set -euo pipefail

APP_NAME="dns-switch"
APP_DIR="build/appdir"

echo "==> Compiling Go binary (CGO_ENABLED=0)..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o "${APP_DIR}/${APP_NAME}" .

echo "==> Running linuxdeploy with GTK plugin..."
linuxdeploy --appdir "${APP_DIR}" \
  --plugin gtk \
  --output appimage

echo "==> Done: ${APP_NAME}-x86_64.AppImage"
