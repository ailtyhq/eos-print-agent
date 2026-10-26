#!/bin/sh
# Compila los binarios de una release y sus sumas SHA-256 (se publican en GitHub Releases).
# Uso: ./build-release.sh 1.0.0
set -eu
VERSION="${1:?indica la versión, p. ej. 1.0.0}"
OUT="dist/$VERSION"
rm -rf "$OUT" && mkdir -p "$OUT"
for target in darwin/arm64 darwin/amd64 windows/amd64 linux/amd64 linux/arm64; do
  os="${target%/*}"; arch="${target#*/}"; ext=""; [ "$os" = windows ] && ext=".exe"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X eos-print-agent/internal/agent.Version=$VERSION" \
    -o "$OUT/eos-print-agent-$VERSION-$os-$arch$ext" ./cmd/eos-print-agent
done
(cd "$OUT" && shasum -a 256 eos-print-agent-* > SHA256SUMS)
echo "Release en $OUT"; cat "$OUT/SHA256SUMS"
