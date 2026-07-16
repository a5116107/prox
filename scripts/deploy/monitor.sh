#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/_lib.sh"

RESULTS_FILE=""
FAILURES=0

cleanup_monitor() {
  [[ -z "$RESULTS_FILE" ]] || rm -f -- "$RESULTS_FILE"
}

record_check() {
  local name="$1" ok="$2" detail="${3:-}"
  detail="${detail//$'\t'/ }"
  detail="${detail//$'\r'/ }"
  detail="${detail//$'\n'/ }"
  printf '%s\t%s\t%s\n' "$name" "$ok" "$detail" >>"$RESULTS_FILE"
  [[ "$ok" == "true" ]] || FAILURES=$((FAILURES + 1))
}

render_monitor_result() {
  local status="ok"
  (( FAILURES == 0 )) || status="failed"
  "$PYTHON_BIN" - "$status" "$RESULTS_FILE" <<'PY'
import datetime
import json
import sys

status, path = sys.argv[1:]
checks = []
with open(path, encoding="utf-8") as handle:
    for line in handle:
        name, ok, detail = line.rstrip("\n").split("\t", 2)
        checks.append({"name": name, "ok": ok == "true", "detail": detail})
print(json.dumps({
    "ok": status == "ok",
    "status": status,
    "checked_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "checks": checks,
}, separators=(",", ":")))
PY
}

check_container() {
  local container="$1" state max_restarts
  state="$(docker inspect --format \
    '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}|{{.RestartCount}}' \
    "$container" 2>/dev/null || true)"
  max_restarts="${MONITOR_MAX_CONTAINER_RESTARTS:-3}"
  if container_state_is_healthy "$state" "$max_restarts"; then
    record_check "container:$container" true "$state"
  else
    record_check "container:$container" false "${state:-missing}; max_restarts=$max_restarts"
  fi
}

container_state_is_healthy() {
  local state="$1" max_restarts="$2" status health restarts
  [[ "$max_restarts" =~ ^[0-9]+$ ]] || return 1
  IFS='|' read -r status health restarts <<<"$state"
  [[ "$status" == "running" ]] || return 1
  [[ "$health" == "healthy" || "$health" == "none" ]] || return 1
  [[ "$restarts" =~ ^[0-9]+$ ]] || return 1
  (( restarts <= max_restarts ))
}

disk_state_is_healthy() {
  local used_percent="$1" available_kb="$2" minimum_free_gb="$3"
  local maximum_disk_percent="$4" minimum_kb
  [[ "$used_percent" =~ ^[0-9]+$ \
    && "$available_kb" =~ ^[0-9]+$ \
    && "$minimum_free_gb" =~ ^[0-9]+$ \
    && "$maximum_disk_percent" =~ ^[0-9]+$ ]] || return 1
  (( 10#$maximum_disk_percent <= 100 )) || return 1
  minimum_kb="$((10#$minimum_free_gb * 1024 * 1024))"
  (( 10#$used_percent <= 10#$maximum_disk_percent \
    && 10#$available_kb >= minimum_kb ))
}

check_fresh_file() {
  local name="$1" file="$2" max_age="$3" age
  [[ "$max_age" =~ ^[0-9]+$ ]] || {
    record_check "$name" false "invalid max_age=$max_age"
    return 0
  }
  age="$(file_age_seconds "$file" 2>/dev/null || true)"
  if [[ "$age" =~ ^[0-9]+$ ]] && (( age <= max_age )); then
    record_check "$name" true "file=$(basename "$file") age_seconds=$age"
  else
    record_check "$name" false "file=${file:-missing} age_seconds=${age:-unknown} max_age=$max_age"
  fi
}

main() {
  local body expected_marker actual_marker static_asset adapter_url adapter_body
  local used_percent available_kb minimum_free_gb maximum_disk_percent payload status
  local backup_dir latest_backup restore_state restore_body
  local -a containers

  require_command docker
  require_command curl
  require_command "$PYTHON_BIN"
  require_command df
  require_command find
  require_command sort
  require_command stat
  require_command date
  load_deploy_env

  RESULTS_FILE="$(mktemp)"
  trap cleanup_monitor EXIT
  read -r -a containers <<<"${MONITOR_CONTAINERS:-new-api new-api-proxy new-api-pg new-api-redis new-api-oauth-worker}"
  for container in "${containers[@]}"; do
    check_container "$container"
  done

  body="$(curl --fail --silent --show-error --max-time 5 \
    http://127.0.0.1:3000/api/status 2>/dev/null || true)"
  if printf '%s' "$body" | json_success_response; then
    record_check api_status true success
  else
    record_check api_status false invalid_or_unreachable
  fi

  expected_marker="$(read_env_file_value "$RELEASES_DIR/current.env" RELEASE_TAG)"
  actual_marker="$(curl --fail --silent --show-error --max-time 5 \
    http://127.0.0.1:3000/release-marker.txt 2>/dev/null | tr -d '\r\n' || true)"
  if [[ -n "$expected_marker" && "$actual_marker" == "$expected_marker" ]]; then
    record_check release_marker true "$actual_marker"
  else
    record_check release_marker false "expected=${expected_marker:-missing} actual=${actual_marker:-missing}"
  fi

  static_asset="$(check_newapi_static_assets 2>/dev/null || true)"
  if [[ -n "$static_asset" ]]; then
    record_check static_assets true "$static_asset"
  else
    record_check static_assets false favicon_or_chunk_unreachable
  fi

  if check_agent_image_config >/dev/null 2>&1; then
    record_check image_config_route true protected_and_authorized
  else
    record_check image_config_route false invalid_auth_or_payload
  fi

  adapter_url="$(resolve_hermes_adapter_health_url 2>/dev/null || true)"
  adapter_body="$(check_hermes_adapter_health "$adapter_url" 2>/dev/null || true)"
  if [[ -n "$adapter_url" ]] && printf '%s' "$adapter_body" | json_adapter_uses_newapi; then
    record_check adapter_health true image_source_newapi
  else
    record_check adapter_health false unavailable_or_stale_source
  fi

  if command -v systemctl >/dev/null 2>&1; then
    status="$(systemctl is-active prox-hermes-adapter.service 2>/dev/null || true)"
    if [[ "$status" == "active" ]]; then
      record_check adapter_service true active
    else
      record_check adapter_service false "${status:-unknown}"
    fi
  fi

  backup_dir="${BACKUP_DIR:-/var/backups/prox}"
  latest_backup="$(latest_matching_file "$backup_dir" 'prox-*.tar.age' 2>/dev/null || true)"
  check_fresh_file backup_fresh "$latest_backup" "${MONITOR_BACKUP_MAX_AGE_SECONDS:-129600}"

  restore_state="${RESTORE_DRILL_STATE_FILE:-$backup_dir/restore-drill.latest.json}"
  restore_body="$(cat "$restore_state" 2>/dev/null || true)"
  if printf '%s' "$restore_body" | json_restore_state_response; then
    check_fresh_file restore_drill_fresh "$restore_state" \
      "${MONITOR_RESTORE_MAX_AGE_SECONDS:-3456000}"
  else
    record_check restore_drill_fresh false "invalid_or_missing_state=$restore_state"
  fi

  used_percent="$(df -P "$REPO_ROOT" | awk 'NR==2 {gsub(/%/, "", $5); print $5}')"
  available_kb="$(df -Pk "$REPO_ROOT" | awk 'NR==2 {print $4}')"
  minimum_free_gb="${MONITOR_MIN_FREE_GB:-8}"
  maximum_disk_percent="${MONITOR_MAX_DISK_PERCENT:-85}"
  if disk_state_is_healthy "$used_percent" "$available_kb" \
    "$minimum_free_gb" "$maximum_disk_percent"; then
    record_check disk true "used=${used_percent}% available_kb=$available_kb"
  else
    record_check disk false \
      "used=${used_percent:-unknown}% available_kb=${available_kb:-unknown} min_free_gb=$minimum_free_gb max_used_percent=$maximum_disk_percent"
  fi

  payload="$(render_monitor_result)"
  printf '%s\n' "$payload"
  if [[ -n "${MONITOR_ALERT_WEBHOOK:-}" ]] \
    && { (( FAILURES > 0 )) || [[ "${MONITOR_WEBHOOK_ALWAYS:-0}" == "1" ]]; }; then
    curl --fail --silent --show-error --max-time 10 \
      -H 'Content-Type: application/json' --data-binary "$payload" \
      "$MONITOR_ALERT_WEBHOOK" >/dev/null || log "monitor webhook delivery failed" >&2
  fi
  (( FAILURES == 0 ))
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
