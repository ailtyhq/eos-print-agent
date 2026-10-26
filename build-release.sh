#!/bin/sh
# Compila los binarios de una release y sus sumas SHA-256, para publicarlos en
# https://github.com/ailtyhq/eos-print-agent/releases.
# Los nombres de archivo NO llevan la versión: EOS enlaza a .../releases/latest/download/<archivo>, que siempre
# apunta a la última versión publicada. La versión va dentro del binario (`eos-print-agent version`).
# Uso: ./build-release.sh 1.0.0
set -eu
VERSION="${1:?indica la versión, p. ej. 1.0.0}"
OUT="dist/$VERSION"
rm -rf "$OUT" && mkdir -p "$OUT"
build() { # GOOS GOARCH nombre-del-archivo
  CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -trimpath \
    -ldflags "-s -w -X eos-print-agent/internal/agent.Version=$VERSION" -o "$OUT/$3" ./cmd/eos-print-agent
}
build darwin arm64 eos-print-agent-macos-apple
build darwin amd64 eos-print-agent-macos-intel
build windows amd64 eos-print-agent-windows.exe
build linux amd64 eos-print-agent-linux-amd64
build linux arm64 eos-print-agent-linux-arm64
(cd "$OUT" && shasum -a 256 eos-print-agent-* > SHA256SUMS)
echo "Release $VERSION en $OUT"; cat "$OUT/SHA256SUMS"
