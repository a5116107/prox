#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/_lib.sh"

require_command docker
require_command git
require_command curl
require_command "$PYTHON_BIN"
require_command flock
load_deploy_env

required=(
  SERVER_IP PUBLIC_DOMAIN NEWAPI_IMAGE PG_PASS REDIS_PASS SESSION_SECRET
  SESSION_COOKIE_DOMAIN CRYPTO_SECRET OAUTH_STATE_SIGNING_SECRET HERMES_SHARED_KEY
)
for key in "${required[@]}"; do
  [[ -n "${!key:-}" ]] || die "required variable is empty: $key"
  [[ "${!key}" != change-me* ]] || die "placeholder value remains in $key"
done

if [[ "${ALLOW_DIRTY_RELEASE:-0}" != "1" ]]; then
  git -C "$REPO_ROOT" diff --quiet || die "tracked working tree changes must be committed before release"
  git -C "$REPO_ROOT" diff --cached --quiet || die "staged changes must be committed before release"
fi

compose config --quiet

available_kb="$(df -Pk "$REPO_ROOT" | awk 'NR==2 {print $4}')"
minimum_kb="$(( ${MIN_FREE_GB:-8} * 1024 * 1024 ))"
(( available_kb >= minimum_kb )) || die "less than ${MIN_FREE_GB:-8} GiB is free on the release filesystem"

if [[ "${SKIP_ADAPTER_CHECK:-0}" != "1" ]]; then
  adapter_health_url="$(resolve_hermes_adapter_health_url)" \
    || die "Hermes adapter health address is invalid"
  check_hermes_adapter_health "$adapter_health_url" >/dev/null \
    || die "Hermes adapter health check failed at $adapter_health_url; set SKIP_ADAPTER_CHECK=1 only for adapter bootstrap"
  log "Hermes adapter health check passed at $adapter_health_url"
  adapter_newapi_url="$(resolve_hermes_newapi_base_url)" \
    || die "Hermes New API base URL is invalid"
  check_hermes_newapi_connection "$adapter_newapi_url" >/dev/null \
    || die "Hermes cannot reach the New API ChatOps endpoint at $adapter_newapi_url"
  log "Hermes New API connection passed at $adapter_newapi_url"
fi

log "preflight passed"
df -h "$REPO_ROOT"
docker system df
if docker buildx version >/dev/null 2>&1; then
  docker buildx du || true
fi
