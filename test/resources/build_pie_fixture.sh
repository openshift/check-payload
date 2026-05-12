#!/bin/bash
set -euo pipefail
# Build a minimal s390x PIE Go binary for testing ReadTable section lookup.
# Go 1.26+ emits .gopclntab as a separate section for PIE builds,
# which broke check-payload's section lookup (issue #329).
#
# Uses zig as cross-linker (s390x PIE needs CGO/external linking).
# Install: brew install zig (macOS) or dnf install zig (Fedora)
#
# When ubi9/go-toolset:1.26 becomes available, consider rebuilding with:
#   podman run --platform linux/s390x -v $PWD:/out:Z ubi9/go-toolset:1.26 \
#     bash -c 'cd /tmp && cp /out/main.go . && go mod init fixture && \
#     GOFIPS140=certified go build -buildmode=pie -ldflags="-s -w" -trimpath -o /out/pie_go126_s390x_app .'
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cp "$SCRIPT_DIR/main.go" "$WORK/"
cd "$WORK"
go mod init fixture
CC="zig cc -target s390x-linux-gnu" \
  GOOS=linux GOARCH=s390x CGO_ENABLED=1 \
  GOFIPS140=certified \
  go build -buildmode=pie -ldflags="-s -w" -trimpath \
  -o "$SCRIPT_DIR/pie_go126_s390x_app" .
echo "Built with: $(go version)"
echo "Output: $SCRIPT_DIR/pie_go126_s390x_app"
