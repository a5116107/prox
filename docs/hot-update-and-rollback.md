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
4. Records the active image as `PREVIOUS_IMAGE`.
5. Recreates only `new-api`; PostgreSQL, Redis, proxy, and OAuth Worker stay running.
6. Waits for container health and `/api/status`.
7. Verifies `/release-marker.txt`, the favicon, a hashed frontend asset, the
   quiz route, and protected/authorized image configuration route.
8. Verifies the separate Adapter is healthy and has refreshed its image
   configuration from New API.
9. Restores the previous image automatically if any post-switch check fails.

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

The rollback also switches only `new-api` and repeats live route checks.

After a release or rollback, run the same production probe used by systemd:

```bash
sudo bash scripts/deploy/monitor.sh | jq .
docker inspect new-api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}'
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
