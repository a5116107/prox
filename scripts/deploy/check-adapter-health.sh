#!/usr/bin/env bash

set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/_lib.sh"

require_command curl

health_url="$(resolve_hermes_adapter_health_url)" \
  || die "Hermes adapter health address is invalid"
health_body="$(check_hermes_adapter_health "$health_url")" \
  || die "Hermes adapter health check failed at $health_url"

log "Hermes adapter is healthy at $health_url"
printf '%s\n' "$health_body"

newapi_url="$(resolve_hermes_newapi_base_url)" \
  || die "Hermes New API base URL is invalid"
check_hermes_newapi_connection "$newapi_url" >/dev/null \
  || die "Hermes cannot reach the New API ChatOps endpoint at $newapi_url"
log "Hermes can reach New API at $newapi_url"
