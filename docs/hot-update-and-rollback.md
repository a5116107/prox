# Hot Update And Rollback

## Release transaction

```bash
cd /opt/prox/current
git fetch origin
git switch main
git pull --ff-only
bash scripts/deploy/release.sh
```

`release.sh` performs this transaction:

1. Locks releases and rejects dirty tracked source.
2. Validates environment, Compose, free disk, and Adapter health.
3. Builds `prox-new-api:<UTC>-<commit>` with an embedded release marker.
4. Records the active image as `PREVIOUS_IMAGE`.
5. Recreates only `new-api`; PostgreSQL, Redis, proxy, and OAuth Worker stay running.
6. Waits for container health and `/api/status`.
7. Verifies `/release-marker.txt` and that the quiz route is registered.
8. Restores the previous image automatically if any post-switch check fails.

## Manual rollback

Rollback to the previous image recorded by the latest release:

```bash
bash scripts/deploy/rollback.sh
```

Rollback to a specific retained image:

```bash
bash scripts/deploy/rollback.sh prox-new-api:RELEASE_TAG
```

The rollback also switches only `new-api` and repeats live route checks.

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
