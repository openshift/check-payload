#!/bin/bash
set -euo pipefail
# Build a s390x Go 1.24 binary for testing pclntab header validation.
# The binary must import enough stdlib packages to produce a .gopclntab
# large enough that the LE magic bytes (0xF1FFFFFF) appear naturally in
# the pclntab data - triggering the false-match bug on big-endian.
#
# Uses zig as cross-linker (s390x CGO needs external linking).
# Install: brew install zig (macOS) or dnf install zig (Fedora)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cp "$SCRIPT_DIR/main.go" "$WORK/"
cd "$WORK"
go mod init fixture
CC="zig cc -target s390x-linux-gnu" \
  GOOS=linux GOARCH=s390x CGO_ENABLED=1 \
  go build -a \
  -o "$SCRIPT_DIR/go124_s390x_app" .
echo "Built with: $(go version)"
echo "Output: $SCRIPT_DIR/go124_s390x_app"
