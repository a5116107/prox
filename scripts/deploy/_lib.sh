#!/usr/bin/env bash

set -Eeuo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$DEPLOY_SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$REPO_ROOT/compose.prod.yml}"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env.deploy}"
RELEASES_DIR="${RELEASES_DIR:-$REPO_ROOT/releases}"
NEWAPI_CONTAINER="${NEWAPI_CONTAINER:-new-api}"
PYTHON_BIN="${PYTHON_BIN:-python3}"
HERMES_ADAPTER_ENV_FILE="${HERMES_ADAPTER_ENV_FILE:-/etc/prox/hermes.env}"

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

read_hermes_adapter_env_value() {
  local key="$1" env_file="${HERMES_ADAPTER_ENV_FILE:-/etc/prox/hermes.env}"
  [[ -f "$env_file" ]] || return 0

  (
    unset "$key"
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
    printf '%s' "${!key:-}"
  )
}

resolve_hermes_adapter_health_url() {
  local configured_url configured_host configured_port
  local health_url host port authority

  configured_url="$(read_hermes_adapter_env_value HERMES_ADAPTER_HEALTH_URL)" || return 1
  configured_host="$(read_hermes_adapter_env_value HERMES_ADAPTER_HOST)" || return 1
  configured_port="$(read_hermes_adapter_env_value HERMES_ADAPTER_PORT)" || return 1

  health_url="${HERMES_ADAPTER_HEALTH_URL:-$configured_url}"
  if [[ -n "$health_url" ]]; then
    case "$health_url" in
      http://*|https://*) printf '%s\n' "$health_url" ;;
      *) log "invalid HERMES_ADAPTER_HEALTH_URL: expected http:// or https://" >&2; return 2 ;;
    esac
    return 0
  fi

  host="${HERMES_ADAPTER_HOST:-${configured_host:-127.0.0.1}}"
  port="${HERMES_ADAPTER_PORT:-${configured_port:-18181}}"
  [[ -n "$host" && "$host" != *[[:space:]/]* ]] || {
    log "invalid HERMES_ADAPTER_HOST" >&2
    return 2
  }
  if [[ ! "$port" =~ ^[0-9]+$ ]] || (( port < 1 || port > 65535 )); then
    log "invalid HERMES_ADAPTER_PORT: $port" >&2
    return 2
  fi

  authority="$host"
  if [[ "$host" == *:* && "$host" != \[*\] ]]; then
    authority="[$host]"
  fi
  printf 'http://%s:%s/health\n' "$authority" "$port"
}

check_hermes_adapter_health() {
  local health_url="${1:-}" body
  [[ -n "$health_url" ]] || health_url="$(resolve_hermes_adapter_health_url)" || return 1
  body="$(curl --fail --silent --show-error \
    --max-time "${HERMES_ADAPTER_HEALTH_TIMEOUT:-5}" "$health_url")" || return 1
  [[ "$body" == *'"ok": true'* || "$body" == *'"ok":true'* ]] || {
    log "Hermes adapter returned an unexpected health response" >&2
    return 1
  }
  printf '%s\n' "$body"
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
