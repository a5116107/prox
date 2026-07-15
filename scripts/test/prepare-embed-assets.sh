#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

for frontend in default classic; do
  dist="$REPO_ROOT/web/$frontend/dist"
  if [[ ! -f "$dist/index.html" ]]; then
    mkdir -p "$dist"
    printf '<!doctype html><title>test embed placeholder</title>\n' >"$dist/index.html"
  fi
done
