#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
if [[ -z "${PYTHON_BIN:-}" ]]; then
  for candidate in python3 python; do
    if command -v "$candidate" >/dev/null 2>&1 \
      && "$candidate" --version >/dev/null 2>&1; then
      PYTHON_BIN="$(command -v "$candidate")"
      break
    fi
  done
fi
[[ -n "${PYTHON_BIN:-}" ]] || {
  printf 'FAIL: Python 3 is required\n' >&2
  exit 1
}
export PYTHON_BIN

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_equal() {
  local expected="$1" actual="$2" label="$3"
  [[ "$expected" == "$actual" ]] || fail "$label: expected=$expected actual=$actual"
}

assert_success() {
  local label="$1"
  shift
  "$@" || fail "$label"
}

assert_failure() {
  local label="$1"
  shift
  if "$@"; then
    fail "$label"
  fi
}

# shellcheck disable=SC1091
source "$DEPLOY_DIR/_lib.sh"
declare -F latest_matching_file >/dev/null \
  || fail "latest file selection helper is missing"
declare -F file_age_seconds >/dev/null \
  || fail "file age helper is missing"

printf '%s' '{"success":true}' | json_success_response || fail "valid status response rejected"
if printf '%s' '{"success":false}' | json_success_response; then
  fail "failed status response accepted"
fi
printf '%s' '{"success":true,"data":{"api_key_configured":true}}' \
  | json_image_config_response || fail "valid image config rejected"
if printf '%s' '{"success":true,"data":{"api_key_configured":"yes"}}' \
  | json_image_config_response; then
  fail "invalid image config accepted"
fi
printf '%s' '{"ok":true,"image":{"source":"newapi"}}' \
  | json_adapter_uses_newapi || fail "New API Adapter source rejected"
if printf '%s' '{"ok":true,"image":{"source":"environment"}}' \
  | json_adapter_uses_newapi; then
  fail "fallback Adapter source accepted as live configuration"
fi

asset="$(extract_static_asset '<html><script src="/static/js/index.abc123.js"></script></html>')"
assert_equal "/static/js/index.abc123.js" "$asset" "static asset extraction"
assert_failure "external asset accepted" extract_static_asset \
  '<script src="https://cdn.example.test/index.js"></script>'

# shellcheck disable=SC1091
source "$DEPLOY_DIR/restore-drill.sh"
declare -F extract_validated_archive >/dev/null \
  || fail "safe archive extraction helper is missing"
declare -F verify_backup_checksums >/dev/null \
  || fail "strict checksum verification helper is missing"
declare -F write_restore_state >/dev/null \
  || fail "restore drill state writer is missing"

archive_fixture="$(mktemp -d)"
trap 'rm -rf -- "$archive_fixture"; [[ -z "${RESULTS_FILE:-}" ]] || rm -f -- "$RESULTS_FILE"' EXIT
mkdir -p "$archive_fixture/source" "$archive_fixture/extracted"
printf '%s\n' '{"schema_version":1}' >"$archive_fixture/source/manifest.json"
printf '%s\n' database >"$archive_fixture/source/database.dump"
printf '%s\n' adapter >"$archive_fixture/source/adapter-state.tar"
printf '%s\n' config >"$archive_fixture/source/deploy-config.tar"
(
  cd "$archive_fixture/source"
  sha256sum database.dump adapter-state.tar deploy-config.tar manifest.json >SHA256SUMS
  tar -cf "$archive_fixture/valid.tar" \
    manifest.json SHA256SUMS database.dump adapter-state.tar deploy-config.tar
)
assert_success "valid archive extraction failed" extract_validated_archive \
  "$archive_fixture/valid.tar" "$archive_fixture/extracted"
assert_success "valid backup checksums rejected" verify_backup_checksums \
  "$archive_fixture/extracted"
printf '%s\n' tampered >>"$archive_fixture/extracted/database.dump"
assert_failure "tampered backup accepted" verify_backup_checksums \
  "$archive_fixture/extracted" 2>/dev/null

"$PYTHON_BIN" - "$archive_fixture/link.tar" <<'PY'
import io
import sys
import tarfile

path = sys.argv[1]
expected = ["manifest.json", "SHA256SUMS", "adapter-state.tar", "deploy-config.tar"]
with tarfile.open(path, "w") as archive:
    for name in expected:
        payload = b"fixture\n"
        member = tarfile.TarInfo(name)
        member.size = len(payload)
        archive.addfile(member, io.BytesIO(payload))
    member = tarfile.TarInfo("database.dump")
    member.type = tarfile.SYMTYPE
    member.linkname = "../outside"
    archive.addfile(member)
PY
mkdir -p "$archive_fixture/link-extracted"
assert_failure "archive link member accepted" extract_validated_archive \
  "$archive_fixture/link.tar" "$archive_fixture/link-extracted" 2>/dev/null

restore_state="$archive_fixture/restore-drill.latest.json"
assert_success "restore state write failed" write_restore_state \
  "$restore_state" "prox-fixture.tar.age" 7
printf '%s' "$(cat "$restore_state")" | json_restore_state_response \
  || fail "valid restore state rejected"
printf '%s' '{"ok":false}' | json_restore_state_response \
  && fail "failed restore state accepted"

touch "$archive_fixture/prox-old.tar.age"
sleep 1
touch "$archive_fixture/prox-new.tar.age"
latest="$(latest_matching_file "$archive_fixture" 'prox-*.tar.age')"
assert_equal "$archive_fixture/prox-new.tar.age" "$latest" "latest backup selection"
age_seconds="$(file_age_seconds "$latest")"
[[ "$age_seconds" =~ ^[0-9]+$ ]] || fail "file age is not numeric"

grep -Fxq 'TimeoutStartSec=2h' "$REPO_ROOT/deploy/systemd/prox-backup.service" \
  || fail "backup service does not allow large database dumps to complete"
grep -Fq 'ReadWritePaths=/var/backups/prox /tmp' \
  "$REPO_ROOT/deploy/systemd/prox-restore-drill.service" \
  || fail "restore drill cannot persist its success state"
grep -Fq 'max-size: "50m"' "$REPO_ROOT/compose.prod.yml" \
  || fail "production container logs are unbounded"
grep -Fxq '    copytruncate' "$REPO_ROOT/deploy/logrotate/prox" \
  || fail "host application log rotation is missing"

# shellcheck disable=SC1091
source "$DEPLOY_DIR/cleanup.sh"
PROTECTED_IMAGE_REFS=()
PROTECTED_IMAGE_IDS=()
# shellcheck disable=SC2034  # Read indirectly by image_is_protected.
PROTECTED_IMAGE_REFS["prox-new-api:active"]=1
# shellcheck disable=SC2034  # Read indirectly by image_is_protected.
PROTECTED_IMAGE_IDS["sha256:active"]=1
assert_success "active image reference not protected" image_is_protected \
  "prox-new-api:active" "sha256:other"
assert_success "container image ID not protected" image_is_protected \
  "prox-new-api:alias" "sha256:active"
assert_failure "unrelated image protected" image_is_protected \
  "prox-new-api:expired" "sha256:expired"
assert_success "old timestamp not eligible" older_than_retention \
  "2000-01-01T00:00:00Z" 14
assert_failure "future timestamp eligible" older_than_retention \
  "2999-01-01T00:00:00Z" 14
cleanup_preview="$(APPLY_CLEANUP=0 cleanup_build_cache 14)"
[[ "$cleanup_preview" == *'docker builder prune --force --filter until=336h'* ]] \
  || fail "aged build cache cleanup preview is missing"

# shellcheck disable=SC1091
source "$DEPLOY_DIR/monitor.sh"
assert_success "healthy container state rejected" container_state_is_healthy \
  'running|healthy|0' 0
assert_failure "restart threshold ignored" container_state_is_healthy \
  'running|healthy|2' 1
assert_failure "unhealthy container accepted" container_state_is_healthy \
  'running|unhealthy|0' 3
assert_success "healthy disk state rejected" disk_state_is_healthy \
  80 9437184 8 85
assert_failure "disk usage threshold ignored" disk_state_is_healthy \
  90 9437184 8 85
assert_failure "invalid free-space threshold accepted" disk_state_is_healthy \
  80 9437184 eight 85
assert_failure "disk percentage above 100 accepted" disk_state_is_healthy \
  80 9437184 8 101
RESULTS_FILE="$(mktemp)"
FAILURES=0
record_check api_status true success
record_check disk false threshold_exceeded
payload="$(render_monitor_result)"
assert_equal "1" "$FAILURES" "monitor failure count"
printf '%s' "$payload" | "$PYTHON_BIN" -c '
import json
import sys
payload = json.load(sys.stdin)
assert payload["ok"] is False
assert len(payload["checks"]) == 2
assert payload["checks"][1]["name"] == "disk"
' || fail "monitor JSON is invalid"

for script in "$DEPLOY_DIR"/*.sh "$DEPLOY_DIR"/tests/*.sh; do
  bash -n "$script" || fail "shell syntax check failed: $script"
done

printf 'PASS: production operations scripts\n'
