#!/usr/bin/env sh
# Builds the read-only demo Lambda zip: the SPA embedded into an arm64
# lcatd-lambda bootstrap, plus a grains/ dir the function reads via
# LCATD_BLOB_DIR=/var/task/grains. Point terraform's `lambda_zip` at the output.
#
# Usage: build-zip.sh <grains-dir> <out.zip>
set -eu

grains="${1:?usage: build-zip.sh <grains-dir> <out.zip>}"
out="${2:?usage: build-zip.sh <grains-dir> <out.zip>}"
[ -d "$grains" ] || { echo "grains dir not found: $grains" >&2; exit 1; }

# .../backend (this script lives at backend/deploy/terraform/modules/readonly-demo)
backend="$(CDPATH= cd -- "$(dirname -- "$0")/../../../.." && pwd)"
grains="$(CDPATH= cd -- "$grains" && pwd)"
outdir="$(CDPATH= cd -- "$(dirname -- "$out")" && pwd)"
out="$outdir/$(basename -- "$out")"

echo "building SPA (embeds into the binary)..."
(cd "$backend/ui" && npm ci && npm run build)

stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

echo "building bootstrap (linux/arm64)..."
(cd "$backend" && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o "$stage/bootstrap" ./cmd/lcatd-lambda)
cp -R "$grains" "$stage/grains"

echo "zipping -> $out"
rm -f "$out"
(cd "$stage" && zip -qr "$out" bootstrap grains)
echo "done: $out"
