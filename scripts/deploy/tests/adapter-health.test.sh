#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../_lib.sh"

temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT

assert_equal() {
  local expected="$1" actual="$2" label="$3"
  [[ "$actual" == "$expected" ]] || {
    printf 'FAIL: %s\nexpected: %s\nactual:   %s\n' "$label" "$expected" "$actual" >&2
    exit 1
  }
}

missing_env="$temp_dir/missing.env"
actual="$(HERMES_ADAPTER_ENV_FILE="$missing_env" resolve_hermes_adapter_health_url)"
assert_equal "http://127.0.0.1:18181/health" "$actual" "defaults without an environment file"

cat >"$temp_dir/hermes.env" <<'EOF'
HERMES_ADAPTER_HOST=172.19.0.1
HERMES_ADAPTER_PORT=18182
NEWAPI_INTERNAL_BASE_URL=http://154.9.253.192/
NEWAPI_CHATOPS_BASE_URL=https://unused.example.test
CHATOPS_WEBHOOK_SECRET=adapter-secret
EOF
actual="$(HERMES_ADAPTER_ENV_FILE="$temp_dir/hermes.env" resolve_hermes_adapter_health_url)"
assert_equal "http://172.19.0.1:18182/health" "$actual" "Docker gateway values from hermes.env"

actual="$(HERMES_ADAPTER_ENV_FILE="$temp_dir/hermes.env" \
  HERMES_ADAPTER_HOST=10.0.0.5 HERMES_ADAPTER_PORT=19191 \
  resolve_hermes_adapter_health_url)"
assert_equal "http://10.0.0.5:19191/health" "$actual" "process environment overrides hermes.env"

actual="$(HERMES_ADAPTER_ENV_FILE="$temp_dir/hermes.env" \
  HERMES_ADAPTER_HEALTH_URL=https://adapter.internal/ready \
  resolve_hermes_adapter_health_url)"
assert_equal "https://adapter.internal/ready" "$actual" "explicit health URL override"

actual="$(HERMES_ADAPTER_ENV_FILE="$temp_dir/hermes.env" resolve_hermes_newapi_base_url)"
assert_equal "http://154.9.253.192" "$actual" "New API base URL from hermes.env"

actual="$(HERMES_ADAPTER_ENV_FILE="$temp_dir/hermes.env" \
  NEWAPI_INTERNAL_BASE_URL=https://newapi.internal/ \
  resolve_hermes_newapi_base_url)"
assert_equal "https://newapi.internal" "$actual" "process New API base URL override"

cat >"$temp_dir/chatops-only.env" <<'EOF'
NEWAPI_CHATOPS_BASE_URL=https://chatops.internal/
EOF
actual="$(HERMES_ADAPTER_ENV_FILE="$temp_dir/chatops-only.env" \
  resolve_hermes_newapi_base_url)"
assert_equal "https://chatops.internal" "$actual" "ChatOps base URL fallback"

if HERMES_ADAPTER_ENV_FILE="$missing_env" NEWAPI_INTERNAL_BASE_URL=file:///tmp/new-api \
  resolve_hermes_newapi_base_url >/dev/null 2>&1; then
  printf 'FAIL: invalid New API base URL was accepted\n' >&2
  exit 1
fi

actual="$(HERMES_ADAPTER_ENV_FILE="$missing_env" HERMES_ADAPTER_HOST=2001:db8::1 \
  resolve_hermes_adapter_health_url)"
assert_equal "http://[2001:db8::1]:18181/health" "$actual" "IPv6 host formatting"

if HERMES_ADAPTER_ENV_FILE="$missing_env" HERMES_ADAPTER_PORT=invalid \
  resolve_hermes_adapter_health_url >/dev/null 2>&1; then
  printf 'FAIL: invalid port was accepted\n' >&2
  exit 1
fi

mkdir -p "$temp_dir/bin"
cat >"$temp_dir/bin/curl" <<'EOF'
#!/usr/bin/env sh
printf '%s\n' "$FAKE_CURL_BODY"
EOF
chmod +x "$temp_dir/bin/curl"

actual="$(PATH="$temp_dir/bin:$PATH" FAKE_CURL_BODY='{"ok": true}' \
  check_hermes_adapter_health "http://adapter.test/health")"
assert_equal '{"ok": true}' "$actual" "healthy Adapter response"

if PATH="$temp_dir/bin:$PATH" FAKE_CURL_BODY='{"ok": false}' \
  check_hermes_adapter_health "http://adapter.test/health" >/dev/null 2>&1; then
  printf 'FAIL: unhealthy Adapter response was accepted\n' >&2
  exit 1
fi

PATH="$temp_dir/bin:$PATH" FAKE_CURL_BODY='{"success":true,"data":{}}' \
  HERMES_ADAPTER_ENV_FILE="$temp_dir/hermes.env" \
  check_hermes_newapi_connection >/dev/null \
  || { printf 'FAIL: healthy Adapter New API link was rejected\n' >&2; exit 1; }

if PATH="$temp_dir/bin:$PATH" FAKE_CURL_BODY='{"success":false}' \
  HERMES_ADAPTER_ENV_FILE="$temp_dir/hermes.env" \
  check_hermes_newapi_connection >/dev/null 2>&1; then
  printf 'FAIL: failed Adapter New API link was accepted\n' >&2
  exit 1
fi

printf 'PASS: Hermes adapter health and New API connection\n'
