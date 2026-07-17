#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/_lib.sh"

TAG="${1:-$(date -u +%Y%m%dT%H%M%SZ)-$(git -C "$REPO_ROOT" rev-parse --short=12 HEAD)}"
[[ "$TAG" =~ ^[A-Za-z0-9._-]+$ ]] || die "release tag contains unsupported characters: $TAG"

bash "$SCRIPT_DIR/preflight.sh"
load_deploy_env
acquire_release_lock

IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-prox-new-api}"
NEW_IMAGE="$IMAGE_REPOSITORY:$TAG"
PREVIOUS_CONTAINER="$(active_newapi_container 2>/dev/null || true)"
PREVIOUS_IMAGE="$(current_container_image)"
CANDIDATE_CONTAINER="new-api-$TAG"
PREVIOUS_WORKER_CONTAINER="$(active_newapi_worker_container 2>/dev/null || true)"
WORKER_CONTAINER="new-api-worker-$TAG"
# shellcheck disable=SC2153
RELEASE_DIR="$RELEASES_DIR/$TAG"
mkdir -p "$RELEASE_DIR"

switched=0
rollback_on_error() {
  local status=$?
  trap - ERR
  if (( switched == 1 )); then
    log "release failed; restoring ${PREVIOUS_CONTAINER:-the pre-release state}"
    restore_previous_newapi "$PREVIOUS_CONTAINER" "$CANDIDATE_CONTAINER" "$PREVIOUS_IMAGE" \
      || log "automatic traffic rollback needs operator cleanup"
    restore_previous_newapi_worker "$PREVIOUS_WORKER_CONTAINER" "$WORKER_CONTAINER"
    if [[ -n "$PREVIOUS_IMAGE" ]]; then
      set_env_value "$ENV_FILE" NEWAPI_IMAGE "$PREVIOUS_IMAGE" || true
      ALLOW_LEGACY_QUIZ_ROUTE=1 wait_newapi "" "$PREVIOUS_IMAGE" || true
    fi
  fi
  exit "$status"
}
trap rollback_on_error ERR

log "building $NEW_IMAGE"
docker build --pull \
  --build-arg "RELEASE_MARKER=$TAG" \
  --build-arg "VITE_REACT_APP_ASSET_VERSION=$TAG" \
  --label "org.opencontainers.image.source=https://github.com/a5116107/prox" \
  --label "org.opencontainers.image.revision=$TAG" \
  -t "$NEW_IMAGE" "$REPO_ROOT"

cat >"$RELEASE_DIR/release.env" <<EOF
RELEASE_TAG=$TAG
RELEASE_COMMIT=$(git -C "$REPO_ROOT" rev-parse HEAD)
NEW_IMAGE=$NEW_IMAGE
PREVIOUS_IMAGE=$PREVIOUS_IMAGE
ACTIVE_CONTAINER=$CANDIDATE_CONTAINER
PREVIOUS_CONTAINER=$PREVIOUS_CONTAINER
ACTIVE_WORKER_CONTAINER=$WORKER_CONTAINER
PREVIOUS_WORKER_CONTAINER=$PREVIOUS_WORKER_CONTAINER
CREATED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF

log "starting $CANDIDATE_CONTAINER before draining ${PREVIOUS_CONTAINER:-none}"
switched=1
switch_newapi_image "$NEW_IMAGE" "$CANDIDATE_CONTAINER" "$TAG" "$PREVIOUS_CONTAINER"
switch_newapi_worker "$NEW_IMAGE" "$WORKER_CONTAINER" "$TAG" "$PREVIOUS_WORKER_CONTAINER"

cp "$RELEASE_DIR/release.env" "$RELEASES_DIR/current.env"
chmod 600 "$RELEASES_DIR/current.env" "$ENV_FILE"
switched=0
trap - ERR
remove_retired_newapi_container "$PREVIOUS_CONTAINER" || log "previous API container cleanup deferred"
remove_retired_newapi_container "$PREVIOUS_WORKER_CONTAINER" || log "previous worker cleanup deferred"
log "release active: $NEW_IMAGE"
docker inspect --format 'name={{.Name}} image={{.Config.Image}} id={{.Image}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}} restarts={{.RestartCount}}' "$CANDIDATE_CONTAINER"
docker inspect --format 'worker={{.Name}} image={{.Config.Image}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}} restarts={{.RestartCount}}' "$WORKER_CONTAINER"
