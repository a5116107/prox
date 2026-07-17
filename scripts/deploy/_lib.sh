#!/usr/bin/env bash

set -Eeuo pipefail

DEPLOY_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$DEPLOY_SCRIPT_DIR/../.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$REPO_ROOT/compose.prod.yml}"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/.env.deploy}"
RELEASES_DIR="${RELEASES_DIR:-$REPO_ROOT/releases}"
NEWAPI_CONTAINER="${NEWAPI_CONTAINER:-new-api}"
NEWAPI_PROXY_CONTAINER="${NEWAPI_PROXY_CONTAINER:-new-api-proxy}"
PYTHON_BIN="${PYTHON_BIN:-python3}"
HERMES_ADAPTER_ENV_FILE="${HERMES_ADAPTER_ENV_FILE:-/etc/prox/hermes.env}"
PROXY_RETIRED_WORKER_PIDS=""

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

newapi_origin_url() {
  printf '%s\n' "${NEWAPI_ORIGIN_URL:-http://${SERVER_IP:-127.0.0.1}}"
}

curl_newapi() {
  curl --header "Host: ${PUBLIC_DOMAIN:-localhost}" "$@"
}

check_newapi_static_assets() {
  local base_url html asset
  base_url="$(newapi_origin_url)"
  curl_newapi --fail --silent --show-error --max-time 5 --output /dev/null \
    "$base_url/favicon.ico" || return 1
  html="$(curl_newapi --fail --silent --show-error --max-time 5 "$base_url/")" || return 1
  asset="$(extract_static_asset "$html")" || return 1
  curl_newapi --fail --silent --show-error --max-time 5 --output /dev/null \
    "$base_url$asset" || return 1
  printf '%s\n' "$asset"
}

wait_for_release_marker() {
  local expected_marker="$1" actual_marker="" base_url
  base_url="$(newapi_origin_url)"
  for _ in $(seq 1 "${MARKER_RETRIES:-15}"); do
    actual_marker="$(curl_newapi --fail --silent --show-error --max-time 5 \
      "$base_url/release-marker.txt" 2>/dev/null | tr -d '\r\n' || true)"
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
  local base_url status adapter_auth_value body
  base_url="$(newapi_origin_url)"
  status="$(curl_newapi --silent --show-error --output /dev/null --write-out '%{http_code}' \
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
  body="$(curl_newapi --fail --silent --show-error --max-time 5 \
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

active_newapi_container() {
  local recorded=""
  recorded="$(read_env_file_value "$RELEASES_DIR/current.env" ACTIVE_CONTAINER)"
  if [[ -n "$recorded" ]] && docker inspect "$recorded" >/dev/null 2>&1; then
    printf '%s\n' "$recorded"
    return 0
  fi
  if docker inspect "$NEWAPI_CONTAINER" >/dev/null 2>&1; then
    printf '%s\n' "$NEWAPI_CONTAINER"
    return 0
  fi
  return 1
}

current_container_image() {
  local container=""
  container="$(active_newapi_container 2>/dev/null || true)"
  [[ -n "$container" ]] || return 0
  docker inspect --format '{{.Config.Image}}' "$container" 2>/dev/null || true
}

proxy_mount_source() {
  local destination="$1"
  docker inspect --format '{{range .Mounts}}{{printf "%s\t%s\n" .Destination .Source}}{{end}}' \
    "$NEWAPI_PROXY_CONTAINER" 2>/dev/null \
    | awk -F '\t' -v destination="$destination" '$1 == destination {print $2; exit}'
}

render_proxy_runtime_site() {
  local output="$1" upstream="${2:-new-api}"
  if [[ "$upstream" != "new-api" ]] && ! valid_newapi_container_name "$upstream"; then
    log "invalid New API proxy upstream: $upstream" >&2
    return 1
  fi
  awk -v upstream="$upstream" '
    {
      line = $0
      replacements += gsub(/server new-api:3000 resolve;/, "server " upstream ":3000 resolve;", line)
      print line
    }
    END { if (replacements != 1) exit 42 }
  ' "$REPO_ROOT/proxy/nginx.conf" >"$output"
}

proxy_worker_pids() {
  docker top "$NEWAPI_PROXY_CONTAINER" -eo pid,args 2>/dev/null \
    | awk 'NR > 1 && /nginx: worker process/ {print $1}'
}

wait_for_proxy_workers_to_exit() {
  local retired_pids="${1:-}" timeout_seconds="${2:-${NEWAPI_PROXY_DRAIN_TIMEOUT_SECONDS:-900}}"
  local deadline current_pids pid pending
  [[ -n "$retired_pids" ]] || return 0
  [[ "$timeout_seconds" =~ ^[0-9]+$ ]] || {
    log "NEWAPI_PROXY_DRAIN_TIMEOUT_SECONDS must be numeric" >&2
    return 1
  }
  deadline="$(( $(date +%s) + timeout_seconds ))"
  while (( $(date +%s) <= deadline )); do
    current_pids="$(proxy_worker_pids || true)"
    pending=0
    for pid in $retired_pids; do
      if grep -qx "$pid" <<<"$current_pids"; then
        pending=1
        break
      fi
    done
    (( pending == 1 )) || return 0
    sleep 1
  done
  log "Nginx workers did not drain within ${timeout_seconds}s: $retired_pids" >&2
  return 1
}

sync_proxy_runtime_config() {
  local upstream="${1:-new-api}"
  local runtime_main runtime_site backup_dir rendered_site status=0 body="" changed=0
  local retired_worker_pids="" rollback_worker_pids="" reload_succeeded=0 rollback_succeeded=0
  runtime_main="$(proxy_mount_source /etc/nginx/nginx.conf)"
  runtime_site="$(proxy_mount_source /etc/nginx/conf.d/default.conf)"
  [[ -f "$runtime_main" && -f "$runtime_site" ]] || {
    log "proxy configuration bind mounts are missing" >&2
    return 1
  }
  backup_dir="$(mktemp -d)"
  cp -- "$runtime_main" "$backup_dir/nginx-main.conf"
  cp -- "$runtime_site" "$backup_dir/nginx.conf"
  rendered_site="$backup_dir/nginx.rendered.conf"
  render_proxy_runtime_site "$rendered_site" "$upstream" || status=1

  if ! cmp -s "$REPO_ROOT/proxy/nginx-main.conf" "$runtime_main"; then
    cat "$REPO_ROOT/proxy/nginx-main.conf" >"$runtime_main" || status=1
    changed=1
  fi
  if (( status == 0 )) && ! cmp -s "$rendered_site" "$runtime_site"; then
    cat "$rendered_site" >"$runtime_site" || status=1
    changed=1
  fi
  if (( status == 0 )); then
    docker exec "$NEWAPI_PROXY_CONTAINER" nginx -t >/dev/null 2>&1 || status=1
  fi
  if (( status != 0 )); then
    cat "$backup_dir/nginx-main.conf" >"$runtime_main" || true
    cat "$backup_dir/nginx.conf" >"$runtime_site" || true
    docker exec "$NEWAPI_PROXY_CONTAINER" nginx -t >/dev/null 2>&1 || true
    rm -rf -- "$backup_dir"
    log "proxy configuration validation failed; previous files restored" >&2
    return 1
  fi
  if (( changed == 1 )); then
    retired_worker_pids="$(proxy_worker_pids || true)"
    retired_worker_pids="${retired_worker_pids//$'\n'/ }"
    if docker exec "$NEWAPI_PROXY_CONTAINER" nginx -s reload >/dev/null; then
      reload_succeeded=1
      sleep 1
    else
      status=1
    fi
  fi
  [[ "$(docker inspect --format '{{.State.Status}}' "$NEWAPI_PROXY_CONTAINER" 2>/dev/null || true)" == "running" ]] \
    || status=1
  if (( status == 0 )); then
    body="$(curl_newapi --fail --silent --show-error --max-time 5 \
      "$(newapi_origin_url)/api/status" 2>/dev/null || true)"
    printf '%s' "$body" | json_success_response || status=1
  fi
  if (( status != 0 )); then
    if (( reload_succeeded == 1 )); then
      rollback_worker_pids="$(proxy_worker_pids || true)"
      rollback_worker_pids="${rollback_worker_pids//$'\n'/ }"
    fi
    cat "$backup_dir/nginx-main.conf" >"$runtime_main" || true
    cat "$backup_dir/nginx.conf" >"$runtime_site" || true
    if docker exec "$NEWAPI_PROXY_CONTAINER" nginx -t >/dev/null 2>&1 \
      && docker exec "$NEWAPI_PROXY_CONTAINER" nginx -s reload >/dev/null 2>&1; then
      rollback_succeeded=1
      PROXY_RETIRED_WORKER_PIDS="$rollback_worker_pids"
    elif (( reload_succeeded == 1 )); then
      cat "$REPO_ROOT/proxy/nginx-main.conf" >"$runtime_main" || true
      cat "$rendered_site" >"$runtime_site" || true
      log "proxy rollback reload failed; candidate config retained for retry" >&2
    fi
    rm -rf -- "$backup_dir"
    if (( rollback_succeeded == 1 )); then
      log "proxy reload failed; previous files restored" >&2
    else
      log "proxy reload failed; active config retained for retry" >&2
    fi
    return 1
  fi
  rm -rf -- "$backup_dir"
  PROXY_RETIRED_WORKER_PIDS="$retired_worker_pids"
  if (( changed == 1 )); then
    log "proxy now routes new requests to $upstream; retired workers are draining"
  else
    log "proxy configuration already routes to $upstream"
  fi
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
      body="$(curl_newapi --fail --silent --show-error --max-time 5 \
        "$(newapi_origin_url)/api/status" 2>/dev/null || true)"
      if printf '%s' "$body" | json_success_response; then
        break
      fi
    fi
    sleep 2
  done
  printf '%s' "$body" | json_success_response || return 1

  if [[ -n "$expected_image" ]]; then
    actual_image="$(docker inspect --format '{{.Config.Image}}' "$NEWAPI_CONTAINER" 2>/dev/null || true)"
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

  route_status="$(curl_newapi --silent --output /dev/null --write-out '%{http_code}' --max-time 5 \
    "$(newapi_origin_url)/api/ops/quiz/${COMMUNITY_SITE_ID:-prox}/stats" || true)"
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

valid_newapi_container_name() {
  [[ "$1" =~ ^new-api-[A-Za-z0-9][A-Za-z0-9_.-]{0,100}$ ]]
}

wait_candidate_newapi() {
  local container="$1" expected_image="$2" expected_marker="${3:-}"
  local health="" body="" actual_image="" actual_marker=""
  for _ in $(seq 1 "${HEALTH_RETRIES:-60}"); do
    health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \
      "$container" 2>/dev/null || true)"
    if [[ "$health" == "healthy" || "$health" == "running" ]]; then
      body="$(docker exec "$container" wget -qO- \
        http://127.0.0.1:3000/api/status 2>/dev/null || true)"
      if printf '%s' "$body" | json_success_response; then
        break
      fi
    fi
    sleep 2
  done
  printf '%s' "$body" | json_success_response || return 1
  actual_image="$(docker inspect --format '{{.Config.Image}}' "$container" 2>/dev/null || true)"
  [[ "$actual_image" == "$expected_image" ]] || {
    log "candidate image mismatch: expected=$expected_image actual=${actual_image:-missing}"
    return 1
  }
  if [[ -n "$expected_marker" ]]; then
    actual_marker="$(docker exec "$container" wget -qO- \
      http://127.0.0.1:3000/release-marker.txt 2>/dev/null | tr -d '\r\n' || true)"
    [[ "$actual_marker" == "$expected_marker" ]] || {
      log "candidate marker mismatch: expected=$expected_marker actual=${actual_marker:-missing}"
      return 1
    }
  fi
}

stage_newapi_container() {
  local image="$1" container="$2" node_type="${3:-slave}"
  if ! valid_newapi_container_name "$container"; then
    log "invalid candidate container name: $container" >&2
    return 1
  fi
  if [[ "$node_type" != "master" && "$node_type" != "slave" ]]; then
    log "invalid NODE_TYPE for candidate: $node_type" >&2
    return 1
  fi
  if docker inspect "$container" >/dev/null 2>&1; then
    log "candidate container already exists: $container" >&2
    return 1
  fi
  export NEWAPI_IMAGE="$image"
  compose run -d --no-deps --name "$container" -e "NODE_TYPE=$node_type" new-api >/dev/null
  docker update --restart always "$container" >/dev/null
}

active_newapi_worker_container() {
  local recorded=""
  recorded="$(read_env_file_value "$RELEASES_DIR/current.env" ACTIVE_WORKER_CONTAINER)"
  if [[ -n "$recorded" ]] && docker inspect "$recorded" >/dev/null 2>&1; then
    printf '%s\n' "$recorded"
    return 0
  fi
  return 1
}

restore_previous_newapi_worker() {
  local previous_container="$1" failed_container="$2"
  if [[ -n "$previous_container" ]] && docker inspect "$previous_container" >/dev/null 2>&1; then
    docker update --restart always "$previous_container" >/dev/null 2>&1 || true
    docker start "$previous_container" >/dev/null 2>&1 || true
  fi
  if [[ -n "$failed_container" ]] && docker inspect "$failed_container" >/dev/null 2>&1; then
    docker update --restart no "$failed_container" >/dev/null 2>&1 || true
    docker stop --time 30 "$failed_container" >/dev/null 2>&1 || true
    docker rm "$failed_container" >/dev/null 2>&1 || true
  fi
}

switch_newapi_worker() {
  local image="$1" candidate="$2" expected_marker="${3:-}" previous_container="${4:-}"
  if [[ -n "$previous_container" && "$previous_container" != "$candidate" ]] \
    && docker inspect "$previous_container" >/dev/null 2>&1; then
    docker update --restart no "$previous_container" >/dev/null
    docker stop --time "${NEWAPI_WORKER_DRAIN_TIMEOUT_SECONDS:-60}" "$previous_container" >/dev/null
  fi
  stage_newapi_container "$image" "$candidate" master
  wait_candidate_newapi "$candidate" "$image" "$expected_marker"
}

remove_retired_newapi_container() {
  local container="$1"
  [[ -n "$container" ]] || return 0
  docker inspect "$container" >/dev/null 2>&1 || return 0
  docker update --restart no "$container" >/dev/null 2>&1 || true
  docker stop --time 30 "$container" >/dev/null 2>&1 || true
  docker rm "$container" >/dev/null
}

restore_previous_newapi() {
  local previous_container="$1" failed_container="$2" previous_image="${3:-}"
  local retired_worker_pids="${PROXY_RETIRED_WORKER_PIDS:-}"
  if [[ -n "$previous_container" ]] && docker inspect "$previous_container" >/dev/null 2>&1; then
    docker update --restart always "$previous_container" >/dev/null 2>&1 || true
    docker start "$previous_container" >/dev/null 2>&1 || true
    if [[ -n "$previous_image" ]]; then
      wait_candidate_newapi "$previous_container" "$previous_image" "" || return 1
    fi
    sync_proxy_runtime_config "$previous_container" || return 1
    if [[ -n "$PROXY_RETIRED_WORKER_PIDS" ]]; then
      retired_worker_pids="$PROXY_RETIRED_WORKER_PIDS"
    fi
    NEWAPI_CONTAINER="$previous_container"
  else
    log "previous New API container is unavailable; failed candidate remains active" >&2
    return 1
  fi
  if ! wait_for_proxy_workers_to_exit "$retired_worker_pids"; then
    log "failed candidate retained until its proxied requests finish: $failed_container" >&2
    return 1
  fi
  PROXY_RETIRED_WORKER_PIDS=""
  if [[ -n "$failed_container" ]] && docker inspect "$failed_container" >/dev/null 2>&1; then
    docker update --restart no "$failed_container" >/dev/null 2>&1 || true
    docker stop --time 30 "$failed_container" >/dev/null 2>&1 || true
    docker rm "$failed_container" >/dev/null 2>&1 || true
  fi
}

switch_newapi_image() {
  local image="$1" candidate="$2" expected_marker="${3:-}"
  local previous_container="${4:-}" drain_seconds retired_worker_pids=""
  stage_newapi_container "$image" "$candidate" slave
  wait_candidate_newapi "$candidate" "$image" "$expected_marker"
  sync_proxy_runtime_config "$candidate"
  retired_worker_pids="$PROXY_RETIRED_WORKER_PIDS"
  NEWAPI_CONTAINER="$candidate"
  wait_newapi "$expected_marker" "$image"

  if [[ -n "$previous_container" && "$previous_container" != "$candidate" ]] \
    && docker inspect "$previous_container" >/dev/null 2>&1; then
    wait_for_proxy_workers_to_exit "$retired_worker_pids"
    PROXY_RETIRED_WORKER_PIDS=""
    docker update --restart no "$previous_container" >/dev/null
    drain_seconds="${NEWAPI_DRAIN_TIMEOUT_SECONDS:-900}"
    if [[ ! "$drain_seconds" =~ ^[0-9]+$ ]]; then
      log "NEWAPI_DRAIN_TIMEOUT_SECONDS must be numeric" >&2
      return 1
    fi
    log "draining $previous_container for up to ${drain_seconds}s"
    docker stop --time "$drain_seconds" "$previous_container" >/dev/null
  fi

  set_env_value "$ENV_FILE" NEWAPI_IMAGE "$image"
}

acquire_release_lock() {
  mkdir -p "$RELEASES_DIR"
  exec 9>"$RELEASES_DIR/.deploy.lock"
  flock -n 9 || die "another prox release or rollback is running"
}
