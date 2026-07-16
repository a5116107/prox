#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/_lib.sh"

declare -A PROTECTED_IMAGE_REFS=()
declare -A PROTECTED_IMAGE_IDS=()
APPLY_CLEANUP=0

protect_image() {
  local ref="${1:-}" image_id
  [[ -n "$ref" ]] || return 0
  PROTECTED_IMAGE_REFS["$ref"]=1
  image_id="$(docker image inspect --format '{{.Id}}' "$ref" 2>/dev/null || true)"
  [[ -z "$image_id" ]] || PROTECTED_IMAGE_IDS["$image_id"]=1
}

image_is_protected() {
  local ref="$1" image_id="${2:-}"
  [[ -n "${PROTECTED_IMAGE_REFS[$ref]:-}" ]] && return 0
  [[ -n "$image_id" && -n "${PROTECTED_IMAGE_IDS[$image_id]:-}" ]]
}

load_protected_images() {
  local container ref current_metadata
  while IFS= read -r container; do
    [[ -n "$container" ]] || continue
    ref="$(docker inspect --format '{{.Config.Image}}' "$container" 2>/dev/null || true)"
    protect_image "$ref"
  done < <(docker ps -aq)

  current_metadata="$RELEASES_DIR/current.env"
  protect_image "$(read_env_file_value "$current_metadata" NEW_IMAGE)"
  protect_image "$(read_env_file_value "$current_metadata" PREVIOUS_IMAGE)"
}

older_than_retention() {
  local timestamp="$1" retention_days="$2" created_epoch cutoff_epoch
  created_epoch="$(date -d "$timestamp" +%s 2>/dev/null || true)"
  [[ "$created_epoch" =~ ^[0-9]+$ ]] || return 1
  cutoff_epoch="$(( $(date +%s) - retention_days * 86400 ))"
  (( created_epoch < cutoff_epoch ))
}

remove_image_candidate() {
  local ref="$1"
  if (( APPLY_CLEANUP == 1 )); then
    docker image rm "$ref"
  else
    printf '[dry-run] docker image rm %q\n' "$ref"
  fi
}

release_dir_is_protected() {
  local dir="$1" release_file new_image previous_image
  release_file="$dir/release.env"
  [[ -f "$release_file" ]] || return 1
  new_image="$(read_env_file_value "$release_file" NEW_IMAGE)"
  previous_image="$(read_env_file_value "$release_file" PREVIOUS_IMAGE)"
  [[ -n "$new_image" && -n "${PROTECTED_IMAGE_REFS[$new_image]:-}" ]] \
    || [[ -n "$previous_image" && -n "${PROTECTED_IMAGE_REFS[$previous_image]:-}" ]]
}

remove_release_dir() {
  local dir="$1" releases_root resolved
  releases_root="$(readlink -f "$RELEASES_DIR")"
  resolved="$(readlink -f "$dir")"
  [[ -n "$releases_root" && "$resolved" == "$releases_root"/* ]] \
    || die "refusing to remove path outside releases directory: $dir"
  if (( APPLY_CLEANUP == 1 )); then
    rm -rf -- "$resolved"
    log "removed expired release metadata: $resolved"
  else
    printf '[dry-run] rm -rf -- %q\n' "$resolved"
  fi
}

cleanup_build_cache() {
  local retention_days="$1" retention_hours
  [[ "$retention_days" =~ ^[0-9]+$ ]] \
    || die "RELEASE_RETENTION_DAYS must be a non-negative integer"
  retention_hours="$((retention_days * 24))"
  if (( APPLY_CLEANUP == 1 )); then
    docker builder prune --force --filter "until=${retention_hours}h"
  else
    printf '[dry-run] docker builder prune --force --filter %s\n' "until=${retention_hours}h"
  fi
}

usage() {
  printf 'Usage: %s [--dry-run|--apply]\n' "$0"
}

main() {
  local arg image_repository retention_days ref image_id created dir
  for arg in "$@"; do
    case "$arg" in
      --dry-run) APPLY_CLEANUP=0 ;;
      --apply) APPLY_CLEANUP=1 ;;
      --help|-h) usage; return 0 ;;
      *) die "unknown cleanup argument: $arg" ;;
    esac
  done

  require_command docker
  require_command date
  require_command readlink
  require_command flock
  load_deploy_env
  acquire_release_lock
  image_repository="${IMAGE_REPOSITORY:-prox-new-api}"
  retention_days="${RELEASE_RETENTION_DAYS:-14}"
  [[ "$retention_days" =~ ^[0-9]+$ ]] || die "RELEASE_RETENTION_DAYS must be a non-negative integer"
  load_protected_images

  while IFS= read -r ref; do
    [[ -n "$ref" && "$ref" != *':<none>' ]] || continue
    image_id="$(docker image inspect --format '{{.Id}}' "$ref" 2>/dev/null || true)"
    image_is_protected "$ref" "$image_id" && continue
    created="$(docker image inspect --format '{{.Created}}' "$ref" 2>/dev/null || true)"
    older_than_retention "$created" "$retention_days" || continue
    remove_image_candidate "$ref"
  done < <(docker image ls --filter "reference=$image_repository:*" \
    --format '{{.Repository}}:{{.Tag}}' | sort -u)

  while IFS= read -r -d '' dir; do
    release_dir_is_protected "$dir" && continue
    older_than_retention "$(stat -c '%y' "$dir")" "$retention_days" || continue
    remove_release_dir "$dir"
  done < <(find "$RELEASES_DIR" -mindepth 1 -maxdepth 1 -type d -print0)

  case "${BUILD_CACHE_PRUNE:-1}" in
    1) cleanup_build_cache "$retention_days" ;;
    0) ;;
    *) die "BUILD_CACHE_PRUNE must be 0 or 1" ;;
  esac

  log "cleanup complete (apply=$APPLY_CLEANUP); active, container-owned, and rollback images were retained"
  df -h "$REPO_ROOT"
  docker system df
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
