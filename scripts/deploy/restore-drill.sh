#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/_lib.sh"

DRILL_DIR=""
DRILL_CONTAINER=""

cleanup_restore_drill() {
  if [[ -n "$DRILL_CONTAINER" ]]; then
    docker rm -fv "$DRILL_CONTAINER" >/dev/null 2>&1 || true
  fi
  [[ -z "$DRILL_DIR" ]] || rm -rf -- "$DRILL_DIR"
}

extract_validated_archive() {
  local archive="$1" output_dir="$2"
  "$PYTHON_BIN" - "$archive" "$output_dir" <<'PY'
import os
import shutil
import sys
import tarfile

archive_path, output_dir = sys.argv[1:]
expected = {
    "manifest.json",
    "SHA256SUMS",
    "database.dump",
    "adapter-state.tar",
    "deploy-config.tar",
}

with tarfile.open(archive_path, mode="r:") as archive:
    members = archive.getmembers()
    names = [member.name for member in members]
    if len(members) != len(expected) or set(names) != expected:
        raise SystemExit("backup archive contains missing, duplicate, or unexpected paths")
    if any(not member.isfile() for member in members):
        raise SystemExit("backup archive contains a non-regular member")

    for member in members:
        source = archive.extractfile(member)
        if source is None:
            raise SystemExit(f"failed to read backup member: {member.name}")
        destination = os.path.join(output_dir, member.name)
        descriptor = os.open(
            destination,
            os.O_WRONLY | os.O_CREAT | os.O_EXCL,
            0o600,
        )
        with source, os.fdopen(descriptor, "wb") as target:
            shutil.copyfileobj(source, target)
PY
}

verify_backup_checksums() {
  local extracted_dir="$1"
  "$PYTHON_BIN" - "$extracted_dir" <<'PY'
import hashlib
import hmac
import os
import re
import sys

root = sys.argv[1]
expected = {
    "database.dump",
    "adapter-state.tar",
    "deploy-config.tar",
    "manifest.json",
}
checksum_pattern = re.compile(r"^([0-9a-f]{64}) ([ *])([A-Za-z0-9._-]+)$")
checksums = {}
with open(os.path.join(root, "SHA256SUMS"), encoding="ascii") as handle:
    for raw_line in handle:
        line = raw_line.rstrip("\n")
        match = checksum_pattern.fullmatch(line)
        if match is None or match.group(3) in checksums:
            raise SystemExit("invalid checksum manifest")
        checksums[match.group(3)] = match.group(1)
if set(checksums) != expected:
    raise SystemExit("checksum manifest is incomplete or contains unexpected paths")

for name, expected_digest in checksums.items():
    digest = hashlib.sha256()
    with open(os.path.join(root, name), "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    if not hmac.compare_digest(digest.hexdigest(), expected_digest):
        raise SystemExit(f"checksum mismatch: {name}")
PY
}

write_restore_state() {
  local state_file="$1" backup_name="$2" table_count="$3"
  "$PYTHON_BIN" - "$state_file" "$backup_name" "$table_count" <<'PY'
import datetime
import json
import os
import stat
import sys
import tempfile

path, backup_name, table_count = sys.argv[1:]
directory = os.path.dirname(os.path.abspath(path))
os.makedirs(directory, mode=0o700, exist_ok=True)
descriptor, temporary = tempfile.mkstemp(prefix=".restore-state.", dir=directory, text=True)
try:
    with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
        json.dump({
            "ok": True,
            "backup": backup_name,
            "public_table_count": int(table_count),
            "completed_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        }, handle, separators=(",", ":"))
        handle.write("\n")
        handle.flush()
        os.fsync(handle.fileno())
    os.chmod(temporary, stat.S_IRUSR | stat.S_IWUSR)
    os.replace(temporary, path)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
PY
}

main() {
  local backup_file="${1:-}" backup_dir identity archive expected_tables actual_tables
  local restore_state_file
  local postgres_image ready=0

  require_command docker
  require_command age
  require_command tar
  require_command flock
  require_command find
  require_command sort
  require_command "$PYTHON_BIN"
  load_deploy_env

  backup_dir="${BACKUP_DIR:-/var/backups/prox}"
  restore_state_file="${RESTORE_DRILL_STATE_FILE:-$backup_dir/restore-drill.latest.json}"
  identity="${BACKUP_AGE_IDENTITY_FILE:-}"
  [[ -n "$identity" && -r "$identity" ]] || die "BACKUP_AGE_IDENTITY_FILE is missing or unreadable"
  if [[ -z "$backup_file" ]]; then
    backup_file="$(latest_matching_file "$backup_dir" 'prox-*.tar.age')"
  fi
  [[ -n "$backup_file" && -f "$backup_file" ]] || die "encrypted backup is missing"
  case "$(basename "$backup_file")" in
    prox-*.tar.age) ;;
    *) die "unexpected backup file name: $backup_file" ;;
  esac

  mkdir -p "$backup_dir"
  exec 8>"$backup_dir/.restore-drill.lock"
  flock -n 8 || die "another restore drill is running"
  umask 077
  DRILL_DIR="$(mktemp -d "${TMPDIR:-/tmp}/prox-restore.XXXXXX")"
  DRILL_CONTAINER="prox-restore-drill-$$-$RANDOM"
  trap cleanup_restore_drill EXIT
  archive="$DRILL_DIR/backup.tar"

  age --decrypt -i "$identity" -o "$archive" "$backup_file"
  extract_validated_archive "$archive" "$DRILL_DIR"
  rm -f -- "$archive"
  verify_backup_checksums "$DRILL_DIR"
  tar -tf "$DRILL_DIR/adapter-state.tar" >/dev/null
  tar -tf "$DRILL_DIR/deploy-config.tar" >/dev/null

  expected_tables="$("$PYTHON_BIN" - "$DRILL_DIR/manifest.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    payload = json.load(handle)
if payload.get("schema_version") != 1:
    raise SystemExit("unsupported manifest schema")
print(payload["postgres"]["public_table_count"])
PY
)"
  [[ "$expected_tables" =~ ^[0-9]+$ ]] || die "manifest table count is invalid"

  postgres_image="${RESTORE_DRILL_POSTGRES_IMAGE:-postgres:15-alpine}"
  docker run -d --rm --network none --name "$DRILL_CONTAINER" \
    -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=restore \
    "$postgres_image" >/dev/null
  for _ in $(seq 1 "${RESTORE_DRILL_RETRIES:-30}"); do
    if docker exec "$DRILL_CONTAINER" pg_isready -U postgres -d restore >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done
  (( ready == 1 )) || die "isolated PostgreSQL restore container did not become ready"
  docker exec -i "$DRILL_CONTAINER" pg_restore -U postgres -d restore \
    --exit-on-error --no-owner --no-privileges <"$DRILL_DIR/database.dump"
  actual_tables="$(docker exec "$DRILL_CONTAINER" psql -At -U postgres -d restore \
    -c "SELECT count(*) FROM pg_tables WHERE schemaname = 'public';" | tr -d '\r\n')"
  [[ "$actual_tables" == "$expected_tables" ]] \
    || die "restored table count mismatch: expected=$expected_tables actual=$actual_tables"

  write_restore_state "$restore_state_file" "$(basename "$backup_file")" "$actual_tables"
  printf '{"ok":true,"backup":"%s","public_table_count":%s}\n' \
    "$(basename "$backup_file")" "$actual_tables"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
