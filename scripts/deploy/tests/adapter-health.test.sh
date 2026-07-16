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

printf 'PASS: Hermes adapter health address resolution\n'
