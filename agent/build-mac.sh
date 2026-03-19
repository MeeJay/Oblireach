#!/usr/bin/env bash
# =============================================================================
# Oblireach Agent — Native macOS build
#
# The darwin stubs (capture_darwin.go, encode_darwin.go, input_darwin.go) have
# no CGO — the agent compiles cleanly on macOS without C dependencies.
# Builds both arm64 (native) and amd64 (cross via clang -arch) in one pass.
#
# Usage (run from the agent/ directory):
#   bash build-mac.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

NATIVE_GOARCH="$(go env GOARCH 2>/dev/null || uname -m | sed 's/x86_64/amd64/')"
case "$NATIVE_GOARCH" in
  arm64) CROSS_GOARCH="amd64"; CROSS_CLANG_ARCH="x86_64" ;;
  amd64) CROSS_GOARCH="arm64"; CROSS_CLANG_ARCH="arm64"  ;;
  *)
    echo "ERROR: Unsupported architecture: $NATIVE_GOARCH" >&2
    exit 1
    ;;
esac

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

echo "Building Oblireach Agent $VERSION darwin/$NATIVE_GOARCH (native)..."
CGO_ENABLED=1 GOOS=darwin GOARCH="$NATIVE_GOARCH" \
  go build -ldflags="-s -w -X main.agentVersion=$VERSION" \
  -o "$OUT_DIR/oblireach-agent-darwin-$NATIVE_GOARCH" .
echo "  -> $OUT_DIR/oblireach-agent-darwin-$NATIVE_GOARCH"

echo ""
echo "Building Oblireach Agent $VERSION darwin/$CROSS_GOARCH (cross, clang -arch $CROSS_CLANG_ARCH)..."
if CGO_ENABLED=1 GOOS=darwin GOARCH="$CROSS_GOARCH" \
     CGO_CFLAGS="-arch $CROSS_CLANG_ARCH" \
     CGO_LDFLAGS="-arch $CROSS_CLANG_ARCH" \
     go build -ldflags="-s -w -X main.agentVersion=$VERSION" \
     -o "$OUT_DIR/oblireach-agent-darwin-$CROSS_GOARCH" . 2>&1; then
  echo "  -> $OUT_DIR/oblireach-agent-darwin-$CROSS_GOARCH"
else
  echo "  WARNING: Cross-compilation darwin/$CROSS_GOARCH echouee (ignoree)."
fi

echo ""
ls -lh "$OUT_DIR"/oblireach-agent-darwin-* 2>/dev/null || true
