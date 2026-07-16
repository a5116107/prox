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

read_env_file_value() {
  local file="$1" key="$2"
  [[ -f "$file" ]] || return 0
  (
    unset "$key"
    set -a
    # shellcheck disable=SC1090
    source "$file"
    set +a
    printf '%s' "${!key:-}"
  )
}

latest_matching_file() {
  local directory="$1" pattern="$2"
  [[ -d "$directory" ]] || return 1
  find "$directory" -maxdepth 1 -type f -name "$pattern" \
    -printf '%T@\t%p\n' | sort -nr | sed -n '1p' | cut -f 2-
}

file_age_seconds() {
  local file="$1" modified_at now
  [[ -f "$file" ]] || return 1
  modified_at="$(stat -c '%Y' "$file" 2>/dev/null || true)"
  now="$(date +%s)"
  [[ "$modified_at" =~ ^[0-9]+$ && "$now" =~ ^[0-9]+$ ]] || return 1
  (( now >= modified_at )) || return 1
  printf '%s\n' "$((now - modified_at))"
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

json_success_response() {
  "$PYTHON_BIN" -c '
import json
import sys

try:
    payload = json.load(sys.stdin)
except (json.JSONDecodeError, OSError):
    raise SystemExit(1)
raise SystemExit(0 if payload.get("success") is True else 1)
'
}

json_image_config_response() {
  "$PYTHON_BIN" -c '
import json
import sys

try:
    payload = json.load(sys.stdin)
    data = payload.get("data") or {}
except (json.JSONDecodeError, OSError, AttributeError):
    raise SystemExit(1)
valid = payload.get("success") is True and isinstance(data.get("api_key_configured"), bool)
raise SystemExit(0 if valid else 1)
'
}

json_adapter_uses_newapi() {
  "$PYTHON_BIN" -c '
import json
import sys

try:
    payload = json.load(sys.stdin)
    image = payload.get("image") or {}
except (json.JSONDecodeError, OSError, AttributeError):
    raise SystemExit(1)
valid = payload.get("ok") is True and image.get("source") == "newapi"
raise SystemExit(0 if valid else 1)
'
}

json_restore_state_response() {
  "$PYTHON_BIN" -c '
import json
import sys

try:
    payload = json.load(sys.stdin)
except (json.JSONDecodeError, OSError):
    raise SystemExit(1)
valid = (
    payload.get("ok") is True
    and isinstance(payload.get("backup"), str)
    and payload["backup"].startswith("prox-")
    and payload["backup"].endswith(".tar.age")
    and isinstance(payload.get("public_table_count"), int)
    and payload["public_table_count"] >= 0
    and isinstance(payload.get("completed_at"), str)
    and bool(payload["completed_at"])
)
raise SystemExit(0 if valid else 1)
'
}

extract_static_asset() {
  local html="${1:-}" asset=""
  asset="$(printf '%s' "$html" \
    | grep -Eo '(src|href)="[^"?]+\.(js|css)(\?[^" ]*)?"' \
    | sed -E 's/^(src|href)="([^"]+)"$/\2/' \
    | head -n 1 || true)"
  case "$asset" in
    /static/*) printf '%s\n' "$asset" ;;
    static/*) printf '/%s\n' "$asset" ;;
    *) return 1 ;;
  esac
}

check_newapi_static_assets() {
  local base_url="http://127.0.0.1:3000" html asset
  curl --fail --silent --show-error --max-time 5 --output /dev/null \
    "$base_url/favicon.ico" || return 1
  html="$(curl --fail --silent --show-error --max-time 5 "$base_url/")" || return 1
  asset="$(extract_static_asset "$html")" || return 1
  curl --fail --silent --show-error --max-time 5 --output /dev/null \
    "$base_url$asset" || return 1
  printf '%s\n' "$asset"
}

wait_for_release_marker() {
  local expected_marker="$1" actual_marker=""
  for _ in $(seq 1 "${MARKER_RETRIES:-15}"); do
    actual_marker="$(curl --fail --silent --show-error --max-time 5 \
      http://127.0.0.1:3000/release-marker.txt 2>/dev/null | tr -d '\r\n' || true)"
    if [[ "$actual_marker" == "$expected_marker" ]]; then
      return 0
    fi
    sleep 2
  done
  log "release marker mismatch: expected=$expected_marker actual=${actual_marker:-missing}"
  return 1
}

quiz_route_status_is_acceptable() {
  local status="$1"
  case "$status" in
    200|401|403) return 0 ;;
    404) [[ "${ALLOW_LEGACY_QUIZ_ROUTE:-0}" == "1" ]] ;;
    *) return 1 ;;
  esac
}

check_agent_image_config() {
  local base_url="http://127.0.0.1:3000" status adapter_auth_value body
  status="$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
    --max-time 5 "$base_url/api/agent/chatops/image-config?source=qq" || true)"
  case "$status" in
    401|403) ;;
    *) log "image config route is not protected; HTTP $status" >&2; return 1 ;;
  esac

  adapter_auth_value="$(read_hermes_adapter_env_value CHATOPS_WEBHOOK_SECRET)" || return 1
  [[ -n "$adapter_auth_value" ]] || {
    log "CHATOPS_WEBHOOK_SECRET is missing from $HERMES_ADAPTER_ENV_FILE" >&2
    return 1
  }
  [[ "$adapter_auth_value" != *$'\r'* && "$adapter_auth_value" != *$'\n'* ]] || {
    log "CHATOPS_WEBHOOK_SECRET contains an invalid line break" >&2
    return 1
  }
  body="$(curl --fail --silent --show-error --max-time 5 \
    --header @<(printf 'Authorization: Bearer %s\n' "$adapter_auth_value") \
    "$base_url/api/agent/chatops/image-config?source=qq")" || return 1
  printf '%s' "$body" | json_image_config_response
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
  local expected_marker="${1:-}" expected_image="${2:-}" health="" body="" route_status=""
  local actual_image=""
  local adapter_health_url="" adapter_body="" static_asset="" surface_ready=0
  for _ in $(seq 1 "${HEALTH_RETRIES:-60}"); do
    health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$NEWAPI_CONTAINER" 2>/dev/null || true)"
    if [[ "$health" == "healthy" || "$health" == "running" ]]; then
      body="$(curl --fail --silent --show-error --max-time 5 http://127.0.0.1:3000/api/status 2>/dev/null || true)"
      if printf '%s' "$body" | json_success_response; then
        break
      fi
    fi
    sleep 2
  done
  printf '%s' "$body" | json_success_response || return 1

  if [[ -n "$expected_image" ]]; then
    actual_image="$(current_container_image)"
    [[ "$actual_image" == "$expected_image" ]] || {
      log "container image mismatch: expected=$expected_image actual=${actual_image:-missing}"
      return 1
    }
  fi

  if [[ -n "$expected_marker" ]]; then
    if ! wait_for_release_marker "$expected_marker"; then
      [[ "${ALLOW_LEGACY_RELEASE_MARKER:-0}" == "1" ]] || return 1
      log "legacy rollback image has no matching release marker; image identity is verified"
    fi
  fi

  route_status="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 5 \
    "http://127.0.0.1:3000/api/ops/quiz/${COMMUNITY_SITE_ID:-prox}/stats" || true)"
  quiz_route_status_is_acceptable "$route_status" || {
    log "quiz API route smoke failed with HTTP $route_status"
    return 1
  }

  adapter_health_url="$(resolve_hermes_adapter_health_url)" || return 1
  for _ in $(seq 1 "${SURFACE_RETRIES:-15}"); do
    static_asset="$(check_newapi_static_assets 2>/dev/null || true)"
    adapter_body="$(check_hermes_adapter_health "$adapter_health_url" 2>/dev/null || true)"
    if [[ -n "$static_asset" ]] \
      && check_agent_image_config >/dev/null 2>&1 \
      && printf '%s' "$adapter_body" | json_adapter_uses_newapi; then
      surface_ready=1
      break
    fi
    sleep 2
  done
  (( surface_ready == 1 )) || {
    log "live surface check failed: static assets, image config, or Adapter source is not ready"
    return 1
  }
  log "live surface ready: asset=$static_asset adapter_source=newapi"
}

switch_newapi_image() {
  local image="$1"
  set_env_value "$ENV_FILE" NEWAPI_IMAGE "$image"
  export NEWAPI_IMAGE="$image"
  compose up -d --no-deps --force-recreate new-api
}

acquire_release_lock() {
  mkdir -p "$RELEASES_DIR"
  exec 9>"$RELEASES_DIR/.deploy.lock"
  flock -n 9 || die "another prox release or rollback is running"
}
