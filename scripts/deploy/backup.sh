#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/_lib.sh"

STAGING_DIR=""
OUTPUT_TMP=""

cleanup_backup() {
  [[ -z "$STAGING_DIR" ]] || rm -rf -- "$STAGING_DIR"
  [[ -z "$OUTPUT_TMP" ]] || rm -f -- "$OUTPUT_TMP"
}

prune_local_backups() {
  local backup_dir="$1" retention_days="$2" old_file
  [[ "$retention_days" =~ ^[0-9]+$ ]] || die "BACKUP_RETENTION_DAYS must be a non-negative integer"
  while IFS= read -r -d '' old_file; do
    rm -f -- "$old_file"
    log "removed expired encrypted backup: $old_file"
  done < <(find "$backup_dir" -maxdepth 1 -type f -name 'prox-*.tar.age' \
    -mtime "+$retention_days" -print0)
}

main() {
  local backup_dir timestamp archive_name final_path table_count active_image commit
  local adapter_state_dir config_dir recipient_file recipient
  local -a age_args

  require_command docker
  require_command age
  require_command tar
  require_command sha256sum
  require_command flock
  require_command "$PYTHON_BIN"
  load_deploy_env

  backup_dir="${BACKUP_DIR:-/var/backups/prox}"
  adapter_state_dir="${HERMES_STATE_DIR:-/var/lib/prox-hermes}"
  recipient_file="${BACKUP_AGE_RECIPIENTS_FILE:-}"
  recipient="${BACKUP_AGE_RECIPIENT:-}"
  if [[ -n "$recipient_file" ]]; then
    [[ -r "$recipient_file" ]] || die "age recipients file is unreadable: $recipient_file"
    age_args=(-R "$recipient_file")
  elif [[ -n "$recipient" ]]; then
    age_args=(-r "$recipient")
  else
    die "set BACKUP_AGE_RECIPIENT or BACKUP_AGE_RECIPIENTS_FILE"
  fi

  [[ -d "$adapter_state_dir" ]] || die "Adapter state directory is missing: $adapter_state_dir"
  [[ -r "$HERMES_ADAPTER_ENV_FILE" ]] || die "Adapter environment is unreadable: $HERMES_ADAPTER_ENV_FILE"
  [[ -r "$RELEASES_DIR/current.env" ]] || die "release metadata is missing: $RELEASES_DIR/current.env"
  mkdir -p "$backup_dir"
  chmod 700 "$backup_dir"
  exec 8>"$backup_dir/.backup.lock"
  flock -n 8 || die "another prox backup is running"

  umask 077
  STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/prox-backup.XXXXXX")"
  trap cleanup_backup EXIT
  config_dir="$STAGING_DIR/config"
  mkdir -p "$config_dir"

  log "dumping PostgreSQL in custom format"
  compose exec -T postgres pg_dump -U newapi -d new-api \
    --format=custom --no-owner --no-acl >"$STAGING_DIR/database.dump"
  table_count="$(compose exec -T postgres psql -At -U newapi -d new-api \
    -c "SELECT count(*) FROM pg_tables WHERE schemaname = 'public';" | tr -d '\r\n')"
  [[ "$table_count" =~ ^[0-9]+$ ]] || die "failed to read PostgreSQL table count"

  tar --numeric-owner -C "$(dirname "$adapter_state_dir")" -cpf \
    "$STAGING_DIR/adapter-state.tar" "$(basename "$adapter_state_dir")"
  cp -- "$ENV_FILE" "$config_dir/.env.deploy"
  cp -- "$HERMES_ADAPTER_ENV_FILE" "$config_dir/hermes.env"
  cp -- "$RELEASES_DIR/current.env" "$config_dir/current.env"
  cp -- "$COMPOSE_FILE" "$config_dir/compose.prod.yml"
  chmod 600 "$config_dir"/* "$config_dir/.env.deploy"
  tar -C "$config_dir" -cpf "$STAGING_DIR/deploy-config.tar" .

  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  archive_name="prox-$timestamp.tar.age"
  final_path="$backup_dir/$archive_name"
  OUTPUT_TMP="$final_path.tmp.$$"
  active_image="$(current_container_image)"
  commit="$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || printf unknown)"

  "$PYTHON_BIN" - "$STAGING_DIR/manifest.json" "$timestamp" "$table_count" \
    "$active_image" "$commit" <<'PY'
import json
import sys

path, created_at, table_count, active_image, commit = sys.argv[1:]
with open(path, "w", encoding="utf-8") as handle:
    json.dump({
        "schema_version": 1,
        "created_at": created_at,
        "postgres": {"image_major": 15, "public_table_count": int(table_count)},
        "active_image": active_image,
        "commit": commit,
        "artifacts": ["database.dump", "adapter-state.tar", "deploy-config.tar"],
    }, handle, separators=(",", ":"))
    handle.write("\n")
PY

  (
    cd "$STAGING_DIR"
    sha256sum database.dump adapter-state.tar deploy-config.tar manifest.json >SHA256SUMS
    tar -cf - manifest.json SHA256SUMS database.dump adapter-state.tar deploy-config.tar \
      | age "${age_args[@]}" -o "$OUTPUT_TMP"
  )
  chmod 600 "$OUTPUT_TMP"
  mv -f -- "$OUTPUT_TMP" "$final_path"
  OUTPUT_TMP=""

  if [[ -n "${BACKUP_RCLONE_DEST:-}" ]]; then
    require_command rclone
    rclone copyto "$final_path" "${BACKUP_RCLONE_DEST%/}/$archive_name"
    if [[ "${BACKUP_RCLONE_PRUNE:-0}" == "1" ]]; then
      rclone delete "$BACKUP_RCLONE_DEST" --min-age "${BACKUP_RETENTION_DAYS:-14}d" \
        --include 'prox-*.tar.age'
    fi
  fi
  if [[ "${BACKUP_PRUNE_LOCAL:-1}" == "1" ]]; then
    prune_local_backups "$backup_dir" "${BACKUP_RETENTION_DAYS:-14}"
  fi
  log "encrypted backup complete: $final_path"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
