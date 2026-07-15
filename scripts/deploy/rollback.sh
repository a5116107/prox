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
log "rolling back only new-api from ${CURRENT_IMAGE:-unknown} to $TARGET_IMAGE"
switch_newapi_image "$TARGET_IMAGE"
wait_newapi "$TARGET_MARKER"

cat >"$RELEASES_DIR/current.env" <<EOF
RELEASE_TAG=${TARGET_MARKER:-rollback}
RELEASE_COMMIT=unknown
NEW_IMAGE=$TARGET_IMAGE
PREVIOUS_IMAGE=$CURRENT_IMAGE
CREATED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF
chmod 600 "$RELEASES_DIR/current.env" "$ENV_FILE"
log "rollback active: $TARGET_IMAGE"
docker inspect --format 'image={{.Config.Image}} id={{.Image}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}} restarts={{.RestartCount}}' "$NEWAPI_CONTAINER"
