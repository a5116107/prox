# Hot Update And Rollback

## Release transaction

```bash
cd /opt/prox/current
git fetch origin
git switch main
git pull --ff-only
set -a
source /etc/prox/operations.env
set +a
bash scripts/deploy/release.sh
```

`release.sh` performs this transaction:

1. Locks releases and rejects dirty tracked source.
2. Validates environment, Compose, free disk, and Adapter health.
3. Builds `prox-new-api:<UTC>-<commit>` with an embedded release marker.
4. Records the active image, traffic container, and worker container.
5. Starts a `NODE_TYPE=slave` candidate without publishing the application port.
6. Verifies the candidate health, image, marker, and `/api/status` before traffic.
7. Backs up the proxy's actual bind-mounted files, renders the verified candidate
   container name into the upstream, validates with `nginx -t`, hot reloads,
   and checks `/api/status` through the public origin.
8. Waits up to `NEWAPI_PROXY_DRAIN_TIMEOUT_SECONDS` for the retired Nginx workers
   to finish existing streams. Only then does it send `SIGTERM` to the previous
   application, whose own drain is bounded by `NEWAPI_DRAIN_TIMEOUT_SECONDS`.
9. Verifies `/release-marker.txt`, the favicon, a hashed frontend asset, the
   quiz route, and protected/authorized image configuration route.
10. Starts one `NODE_TYPE=master` worker after the previous master has exited,
    so candidate warm-up never duplicates scheduled business tasks.
11. Verifies the separate Adapter is healthy and has refreshed its image
   configuration from New API.
12. On failure, starts and verifies the previous traffic container, points a new
    Nginx generation back to it, drains candidate-bound proxy workers, and only
    then removes the failed candidate. The previous worker is restored separately.

The GitHub `Container` workflow is reusable and is called by `Quality` only
after all Go, PostgreSQL concurrency, Python, web, delivery, and secret jobs
succeed for a `main` or `v*` push. Manual image builds remain available through
`workflow_dispatch` for operator-controlled recovery.

## Manual rollback

Rollback to the previous image recorded by the latest release:

```bash
set -a
source /etc/prox/operations.env
set +a
bash scripts/deploy/rollback.sh
```

Rollback to a specific retained image:

```bash
set -a
source /etc/prox/operations.env
set +a
bash scripts/deploy/rollback.sh prox-new-api:RELEASE_TAG
```

Rollback uses the same candidate-first flow and repeats the live route checks.

After a release or rollback, run the same production probe used by systemd:

```bash
sudo bash scripts/deploy/monitor.sh | jq .
set -a; source /etc/prox/operations.env; set +a
active_container="$(sed -n 's/^ACTIVE_CONTAINER=//p' "$RELEASES_DIR/current.env")"
worker_container="$(sed -n 's/^ACTIVE_WORKER_CONTAINER=//p' "$RELEASES_DIR/current.env")"
docker inspect "${active_container:-new-api}" --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}'
docker inspect "$worker_container" --format '{{.Config.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}'
```

## Compatibility rule

Every schema change follows expand, migrate, contract:

1. Add nullable/defaulted columns and deploy code that supports old and new rows.
2. Backfill and verify separately.
3. Remove old fields in a later release after the rollback window closes.

This keeps the previous image usable. Destructive migrations do not belong in a normal release.

## Adapter updates

Adapter state remains under `/var/lib/prox-hermes`. Update its virtual environment only when dependencies change, then restart the unit:

```bash
sudo /opt/prox/venv/bin/pip install -r integrations/newapi-hermes-adapter/requirements.txt
sudo systemctl restart prox-hermes-adapter
sudo bash scripts/deploy/check-adapter-health.sh
```

Core API traffic remains available while Adapter restarts. Schedule Adapter restarts outside active game rounds.

## Retention before large builds

```bash
df -h /
docker system df -v
docker buildx du
sudo bash scripts/deploy/cleanup.sh --dry-run
```

Apply cleanup only outside an active release. The cleanup script takes the same
release lock as deploy/rollback and never invokes a broad Docker prune. It may
remove only dangling build cache older than `RELEASE_RETENTION_DAYS` when
`BUILD_CACHE_PRUNE=1`.
