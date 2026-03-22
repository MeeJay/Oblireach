#!/usr/bin/env bash
# =============================================================================
# Oblireach Agent — Native Linux build
#
# The linux stubs have no CGO — the agent compiles cleanly on Linux without
# C dependencies (screen capture not yet implemented on Linux).
# Builds amd64 only for now.
#
# Usage (run from the agent/ directory):
#   bash build-linux.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

VERSION=""
if [ -f "VERSION" ]; then
  VERSION="$(tr -d '[:space:]' < VERSION)"
fi
if [ -z "$VERSION" ] || [ "$VERSION" = "dev" ]; then
  echo "WARNING: VERSION non trouve ou 'dev'."
  VERSION="dev"
fi

OUT_DIR="dist"
mkdir -p "$OUT_DIR"

echo "Building Oblireach Agent $VERSION linux/amd64 (CGO for X11)..."

# Check if X11 dev headers are available
if pkg-config --exists x11 2>/dev/null; then
  echo "  X11 development headers found — building with screen capture."
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.agentVersion=$VERSION" \
    -o "$OUT_DIR/oblireach-agent-linux-amd64" .
else
  echo "  WARNING: X11 headers not found (install libx11-dev libxrandr-dev libxtst-dev)."
  echo "  Building without screen capture (stubs only)."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -tags nocgo -ldflags="-s -w -X main.agentVersion=$VERSION" \
    -o "$OUT_DIR/oblireach-agent-linux-amd64" .
fi
echo "  -> $OUT_DIR/oblireach-agent-linux-amd64"

echo ""
ls -lh "$OUT_DIR"/oblireach-agent-linux-* 2>/dev/null || true
