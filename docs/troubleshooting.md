# Troubleshooting

## Static chunks or favicon return 502/429

```bash
set -a; source /etc/prox/operations.env; source "$ENV_FILE"; set +a
active_container="$(sed -n 's/^ACTIVE_CONTAINER=//p' "$RELEASES_DIR/current.env")"
docker inspect "${active_container:-new-api}" --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}}'
curl -I -H "Host: $PUBLIC_DOMAIN" "http://$SERVER_IP/favicon.ico"
curl -fsS -H "Host: $PUBLIC_DOMAIN" "http://$SERVER_IP/release-marker.txt"
docker logs --tail 200 new-api-proxy
```

A host-side frontend build is not live. Rebuild/switch the application image. The application excludes GET/HEAD static assets (`/static/`, `/assets/`, `/fonts/`, favicon, logo, scripts, styles, images, and fonts) from the page-navigation limiter. A static 429 therefore points to an outer reverse proxy/CDN rule or an image older than the R4 limiter fix, not an upstream model limit. The API limiter uses a signed session user or credential bucket first and a separate four-times-larger IP umbrella; inspect `X-RateLimit-Scope` and `Retry-After` on a 429 response to identify the bucket.

## API route returns 404

Confirm the active image marker, then request the route on loopback. `401` or `403` proves a protected route is registered; `404` means the active binary lacks it. Release a corrected image instead of copying Go source into the container.

## Upstream requests return 429

Separate layers using response headers and logs:

1. Static file 429: local web/proxy limiter.
2. `/api/*` before channel selection: local global/search/critical limiter.
3. `X-RateLimit-Scope: upstream-channel`: provider limit or account quota; `Retry-After` is the provider value when available, otherwise the configured channel cooldown.
4. Repeated retries: inspect channel retry policy and circuit breaker.

Do not globally disable all limits. Exempt static/internal health paths and tune authenticated API budgets independently.

## Bot does not reply

```bash
systemctl status prox-hermes-adapter --no-pager
journalctl -u prox-hermes-adapter -n 200 --no-pager
sudo bash scripts/deploy/check-adapter-health.sh
docker exec new-api getent hosts host.docker.internal
```

Verify `HERMES_SHARED_KEY` matches `HERMES_ADAPTER_KEY`/`GAME_ADMIN_KEY`, the OneBot/TG endpoint is reachable, and only one consumer owns each message claim. Adapter writable files must be under `/var/lib/prox-hermes`.

## QQ image generation fails

```bash
systemctl show prox-hermes-adapter -p MainPID -p EnvironmentFiles --no-pager
journalctl -u prox-hermes-adapter --since '2 hours ago' --no-pager \
  | grep -E 'image_config_refresh|image_upstream|image_delivery'
grep -E '^(IMAGE_CONFIG_FROM_NEWAPI|IMAGE_CONFIG_CACHE_TTL_SECONDS|IMAGE_CONFIG_FETCH_TIMEOUT_SECONDS|IMAGE_API_BASE_URL|IMAGE_MODEL|IMAGE_SIZE|IMAGE_RETRY_LIMIT)=' \
  /etc/prox/hermes.env
sudo bash -c '
  set -a; source /etc/prox/hermes.env; set +a
  health_url="${HERMES_ADAPTER_HEALTH_URL:-http://${HERMES_ADAPTER_HOST}:${HERMES_ADAPTER_PORT:-18181}/health}"
  curl -fsS "$health_url"
' | jq '.image'
```

`image_config_refresh status=ok source=newapi` proves the Adapter loaded the
admin configuration. `status=fallback` means New API or its ChatOps secret is
unavailable; the Adapter keeps the cached or environment configuration and
retries within five seconds. After an admin save, allow one configured cache
TTL before treating an old value as stale.

An `image_upstream` HTTP `401/403` means the selected image credential must be
rotated. HTTP `502/524` is transient and is retried within the selected retry
limit. `image_delivery status=delivery_error` points to OneBot delivery; the
Adapter sends the generated URL as a fallback so the result is not lost.

## Quiz has no question

Check that the bank has published questions, the bank itself is published, and an exact or wildcard binding matches site/platform/group. Then inspect `/api/ops/quiz/<site>/stats`. The Adapter no longer falls back to a hard-coded question list.

## Task is terminal but billing is pending

Find the row by the public task ID or operation key:

```sql
SELECT operation_key, status, attempt_count, last_error,
       funding_applied, token_applied, token_cache_invalidated,
       usage_applied, log_applied, lease_until, next_attempt_at
FROM task_billing_operations
WHERE task_id = 'TASK_ID'
ORDER BY id DESC;
```

The terminal task and outbox row commit together. A missing row therefore
indicates an old image or a terminal write path outside the finalizer; verify
the active image before changing data. For an existing row, resolve
`last_error` and let the worker replay it. Funding journals and billing logs are
idempotent, so restarting the application is sufficient after a transient
database or log-database outage.

## Release failed

`release.sh` restores the prior image automatically after a failed switch. Inspect:

```bash
set -a; source /etc/prox/operations.env; set +a
cat "$RELEASES_DIR/current.env"
active_container="$(sed -n 's/^ACTIVE_CONTAINER=//p' "$RELEASES_DIR/current.env")"
docker inspect "${active_container:-new-api}" --format '{{.Config.Image}} {{.State.Health.Status}} {{.RestartCount}}'
docker logs --tail 300 "${active_container:-new-api}"
```

Use `bash scripts/deploy/rollback.sh IMAGE` when an explicit retained image is required.

If a release is waiting rather than failing, inspect the retired Nginx workers
before taking action:

```bash
docker top new-api-proxy -eo pid,args | grep 'nginx: worker process'
runtime_site="$(docker inspect new-api-proxy --format '{{range .Mounts}}{{if eq .Destination "/etc/nginx/conf.d/default.conf"}}{{.Source}}{{end}}{{end}}')"
grep '^server new-api-' "$runtime_site"
```

The release keeps the previous application running until the worker PIDs captured
before the hot reload have exited. `NEWAPI_PROXY_DRAIN_TIMEOUT_SECONDS` bounds
that wait; increasing it preserves longer streams without restarting Nginx.
