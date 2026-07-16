# Troubleshooting

## Static chunks or favicon return 502/429

```bash
docker inspect new-api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}}'
curl -I http://127.0.0.1:3000/favicon.ico
curl -fsS http://127.0.0.1:3000/release-marker.txt
docker logs --tail 200 new-api-proxy
```

A host-side frontend build is not live. Rebuild/switch the application image. Static assets must be excluded from API rate limiting at the proxy/application policy; a static 429 is local configuration, not an upstream model limit.

## API route returns 404

Confirm the active image marker, then request the route on loopback. `401` or `403` proves a protected route is registered; `404` means the active binary lacks it. Release a corrected image instead of copying Go source into the container.

## Upstream requests return 429

Separate layers using response headers and logs:

1. Static file 429: local web/proxy limiter.
2. `/api/*` before channel selection: local global/search/critical limiter.
3. Relay response with channel/upstream request ID: provider limit or account quota.
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
  | grep -E '\[Image\]|image_generate|Photo send'
grep -E '^(IMAGE_API_BASE_URL|IMAGE_MODEL|IMAGE_SIZE|IMAGE_RETRY_LIMIT)=' \
  /etc/prox/hermes.env
```

An HTTP `401/403` means the image credential must be rotated. HTTP `502/524`
is transient and is retried within `IMAGE_RETRY_LIMIT`; persistent failures
must remain visible as `image_generate status=error`. A successful generation
with `status=delivery_error` points to OneBot image delivery, and the Adapter
sends the generated URL as a fallback so the result is not lost.

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
cat releases/current.env
docker inspect new-api --format '{{.Config.Image}} {{.State.Health.Status}} {{.RestartCount}}'
docker logs --tail 300 new-api
```

Use `bash scripts/deploy/rollback.sh IMAGE` when an explicit retained image is required.
