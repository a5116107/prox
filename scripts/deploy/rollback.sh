#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/_lib.sh"

require_command docker
require_command curl
require_command python3
require_command flock
load_deploy_env
acquire_release_lock
sync_proxy_runtime_config

CURRENT_CONTAINER="$(active_newapi_container 2>/dev/null || true)"
CURRENT_WORKER_CONTAINER="$(active_newapi_worker_container 2>/dev/null || true)"
CURRENT_IMAGE="$(current_container_image)"
TARGET_IMAGE="${1:-}"
if [[ -z "$TARGET_IMAGE" ]]; then
  [[ -f "$RELEASES_DIR/current.env" ]] || die "release metadata is missing and no target image was supplied"
  # shellcheck disable=SC1091
  source "$RELEASES_DIR/current.env"
  TARGET_IMAGE="${PREVIOUS_IMAGE:-}"
fi
[[ -n "$TARGET_IMAGE" ]] || die "rollback target image is empty"
docker image inspect "$TARGET_IMAGE" >/dev/null

TARGET_MARKER="$(image_release_marker "$TARGET_IMAGE")"
ROLLBACK_CONTAINER="new-api-rollback-$(date -u +%Y%m%dT%H%M%S)-$$"
ROLLBACK_WORKER_CONTAINER="new-api-worker-rollback-$(date -u +%Y%m%dT%H%M%S)-$$"
CURRENT_METADATA_BACKUP="$(mktemp)"
CURRENT_METADATA_PRESENT=0
if [[ -f "$RELEASES_DIR/current.env" ]]; then
  cp -- "$RELEASES_DIR/current.env" "$CURRENT_METADATA_BACKUP"
  CURRENT_METADATA_PRESENT=1
fi
switched=0
rollback_on_error() {
  local status=$?
  trap - ERR
  if (( switched == 1 )); then
    restore_previous_newapi "$CURRENT_CONTAINER" "$ROLLBACK_CONTAINER"
    restore_previous_newapi_worker "$CURRENT_WORKER_CONTAINER" "$ROLLBACK_WORKER_CONTAINER"
    [[ -z "$CURRENT_IMAGE" ]] || set_env_value "$ENV_FILE" NEWAPI_IMAGE "$CURRENT_IMAGE" || true
    if (( CURRENT_METADATA_PRESENT == 1 )); then
      cp -- "$CURRENT_METADATA_BACKUP" "$RELEASES_DIR/current.env" || true
    else
      rm -f -- "$RELEASES_DIR/current.env"
    fi
  fi
  rm -f -- "$CURRENT_METADATA_BACKUP"
  exit "$status"
}
trap rollback_on_error ERR
log "starting rollback candidate before draining ${CURRENT_CONTAINER:-unknown}"
switched=1
ALLOW_LEGACY_RELEASE_MARKER=1 ALLOW_LEGACY_QUIZ_ROUTE=1 \
  switch_newapi_image "$TARGET_IMAGE" "$ROLLBACK_CONTAINER" "$TARGET_MARKER" "$CURRENT_CONTAINER"
ALLOW_LEGACY_RELEASE_MARKER=1 \
  switch_newapi_worker "$TARGET_IMAGE" "$ROLLBACK_WORKER_CONTAINER" "$TARGET_MARKER" "$CURRENT_WORKER_CONTAINER"

cat >"$RELEASES_DIR/current.env" <<EOF
RELEASE_TAG=${TARGET_MARKER:-rollback}
RELEASE_COMMIT=unknown
NEW_IMAGE=$TARGET_IMAGE
PREVIOUS_IMAGE=$CURRENT_IMAGE
ACTIVE_CONTAINER=$ROLLBACK_CONTAINER
PREVIOUS_CONTAINER=$CURRENT_CONTAINER
ACTIVE_WORKER_CONTAINER=$ROLLBACK_WORKER_CONTAINER
PREVIOUS_WORKER_CONTAINER=$CURRENT_WORKER_CONTAINER
CREATED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF
chmod 600 "$RELEASES_DIR/current.env" "$ENV_FILE"
switched=0
trap - ERR
rm -f -- "$CURRENT_METADATA_BACKUP"
remove_retired_newapi_container "$CURRENT_CONTAINER" || log "previous API container cleanup deferred"
remove_retired_newapi_container "$CURRENT_WORKER_CONTAINER" || log "previous worker cleanup deferred"
log "rollback active: $TARGET_IMAGE"
docker inspect --format 'name={{.Name}} image={{.Config.Image}} id={{.Image}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}} restarts={{.RestartCount}}' "$ROLLBACK_CONTAINER"
docker inspect --format 'worker={{.Name}} image={{.Config.Image}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}} restarts={{.RestartCount}}' "$ROLLBACK_WORKER_CONTAINER"
