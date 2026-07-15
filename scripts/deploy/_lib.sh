#!/usr/bin/env bash

set -Eeuo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$DEPLOY_SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$REPO_ROOT/compose.prod.yml}"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env.deploy}"
RELEASES_DIR="${RELEASES_DIR:-$REPO_ROOT/releases}"
NEWAPI_CONTAINER="${NEWAPI_CONTAINER:-new-api}"
PYTHON_BIN="${PYTHON_BIN:-python3}"

log() {
  printf '[prox-deploy] %s\n' "$*"
}

die() {
  printf '[prox-deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command is missing: $1"
}

load_deploy_env() {
  [[ -f "$ENV_FILE" ]] || die "environment file is missing: $ENV_FILE"
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
}

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

set_env_value() {
  local file="$1" key="$2" value="$3"
  "$PYTHON_BIN" - "$file" "$key" "$value" <<'PY'
import os
import re
import stat
import sys
import tempfile

path, key, value = sys.argv[1:]
pattern = re.compile(rf"^[ \t]*(?:export[ \t]+)?{re.escape(key)}[ \t]*=")
with open(path, "r", encoding="utf-8") as handle:
    lines = handle.readlines()

replacement = f"{key}={value}\n"
result = []
replaced = False
for line in lines:
    if pattern.match(line):
        if not replaced:
            result.append(replacement)
            replaced = True
        continue
    result.append(line)
if not replaced:
    if result and not result[-1].endswith("\n"):
        result[-1] += "\n"
    result.append(replacement)

directory = os.path.dirname(os.path.abspath(path))
fd, temp_path = tempfile.mkstemp(prefix=".env.", dir=directory, text=True)
try:
    with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as handle:
        handle.writelines(result)
    os.chmod(temp_path, stat.S_IRUSR | stat.S_IWUSR)
    os.replace(temp_path, path)
finally:
    if os.path.exists(temp_path):
        os.unlink(temp_path)
PY
}

current_container_image() {
  docker inspect --format '{{.Config.Image}}' "$NEWAPI_CONTAINER" 2>/dev/null || true
}

image_release_marker() {
  docker image inspect --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' "$1" 2>/dev/null || true
}

wait_newapi() {
  local expected_marker="${1:-}" health="" body="" actual_marker="" route_status=""
  for _ in $(seq 1 "${HEALTH_RETRIES:-60}"); do
    health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$NEWAPI_CONTAINER" 2>/dev/null || true)"
    if [[ "$health" == "healthy" || "$health" == "running" ]]; then
      body="$(curl --fail --silent --show-error --max-time 5 http://127.0.0.1:3000/api/status 2>/dev/null || true)"
      if [[ "$body" == *'"success":true'* || "$body" == *'"success": true'* ]]; then
        break
      fi
    fi
    sleep 2
  done
  [[ "$body" == *'"success":true'* || "$body" == *'"success": true'* ]] || return 1

  if [[ -n "$expected_marker" ]]; then
    actual_marker="$(curl --fail --silent --show-error --max-time 5 http://127.0.0.1:3000/release-marker.txt 2>/dev/null | tr -d '\r\n' || true)"
    [[ "$actual_marker" == "$expected_marker" ]] || {
      log "release marker mismatch: expected=$expected_marker actual=${actual_marker:-missing}"
      return 1
    }
  fi

  route_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 5 \
    "http://127.0.0.1:3000/api/ops/quiz/${COMMUNITY_SITE_ID:-prox}/stats" || true)"
  case "$route_status" in
    200|401|403) ;;
    *)
      log "quiz API route smoke failed with HTTP $route_status"
      return 1
      ;;
  esac
}

switch_newapi_image() {
  local image="$1"
  set_env_value "$ENV_FILE" NEWAPI_IMAGE "$image"
  compose up -d --no-deps --force-recreate new-api
}

acquire_release_lock() {
  mkdir -p "$RELEASES_DIR"
  exec 9>"$RELEASES_DIR/.deploy.lock"
  flock -n 9 || die "another prox release or rollback is running"
}
