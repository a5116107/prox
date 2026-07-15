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
PREVIOUS_IMAGE="$(current_container_image)"
# shellcheck disable=SC2153
RELEASE_DIR="$RELEASES_DIR/$TAG"
mkdir -p "$RELEASE_DIR"

switched=0
rollback_on_error() {
  local status=$?
  trap - ERR
  if (( switched == 1 )) && [[ -n "$PREVIOUS_IMAGE" ]]; then
    log "release failed; restoring $PREVIOUS_IMAGE"
    switch_newapi_image "$PREVIOUS_IMAGE" || true
    wait_newapi || true
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
CREATED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF

log "switching only new-api from ${PREVIOUS_IMAGE:-none} to $NEW_IMAGE"
switched=1
switch_newapi_image "$NEW_IMAGE"
wait_newapi "$TAG"
switched=0
trap - ERR

cp "$RELEASE_DIR/release.env" "$RELEASES_DIR/current.env"
chmod 600 "$RELEASES_DIR/current.env" "$ENV_FILE"
log "release active: $NEW_IMAGE"
docker inspect --format 'image={{.Config.Image}} id={{.Image}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}} restarts={{.RestartCount}}' "$NEWAPI_CONTAINER"
