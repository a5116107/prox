# Operations

MM Portal and AWX use a separate operations-plane lifecycle. See
[MM Portal And AWX Operations](mm-awx-operations.md) for topology, health
checks, backup validation, rollback, and K3s disk-retention boundaries.

## Daily checks

```bash
sudo bash scripts/deploy/monitor.sh | jq .
systemctl --failed --no-pager
systemctl list-timers 'prox-*' --no-pager
df -h /
docker system df
```

`monitor.sh` fails if a required container is unhealthy, `/api/status` or the
release marker is wrong, the favicon or a hashed frontend asset is unavailable,
the internal image route loses protection, the Adapter does not use the New API
configuration source, a container exceeds the configured restart threshold,
the latest encrypted backup/restore proof is stale, or disk thresholds are exceeded. Set
`MONITOR_ALERT_WEBHOOK` in mode-`0600` `/etc/prox/monitor.env` to receive the
same machine-readable payload on failures.

Review Nginx status counts, API latency, upstream 429s, Bot action failures,
budget pool balance, and risk-control audit events in addition to this probe.

## Asynchronous task billing

Task and Midjourney terminal states use the `task_billing_operations` outbox.
All API replicas run a fenced worker, so do not disable it on secondary nodes.
Inspect pending age and repeated failures during daily checks:

```sql
SELECT status, COUNT(*)
FROM task_billing_operations
WHERE completed_at = 0
GROUP BY status;

SELECT id, operation_key, task_kind, task_id, attempt_count,
       last_error, next_attempt_at, lease_until
FROM task_billing_operations
WHERE completed_at = 0
ORDER BY id
LIMIT 100;
```

A short-lived `processing` row is normal. A row whose lease has expired is
claimed automatically. Investigate durable dependency errors such as exhausted
wallet/subscription quota or database connectivity before retrying; do not edit
step flags or apply quota manually. After resolving the dependency, set only
`next_attempt_at` to `0` when an immediate retry is required. Confirm the row
reaches `completed`, the task remains terminal, and exactly one funding journal
and billing log share the operation key.

## Runtime source verification

Host source and build output are supporting evidence only. Confirm the active
container, embedded release marker, live routes, and separate Adapter service:

```bash
git -C /opt/prox/current remote -v
git -C /opt/prox/current status --short --branch
git -C /opt/prox/current rev-parse HEAD
set -a; source /etc/prox/operations.env; source "$ENV_FILE"; set +a
active_container="$(sed -n 's/^ACTIVE_CONTAINER=//p' "$RELEASES_DIR/current.env")"
worker_container="$(sed -n 's/^ACTIVE_WORKER_CONTAINER=//p' "$RELEASES_DIR/current.env")"
docker inspect "${active_container:-new-api}" --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}'
docker inspect "$worker_container" --format '{{.Config.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}'
docker inspect "${active_container:-new-api}" --format '{{json .Mounts}}'
curl -fsS -H "Host: $PUBLIC_DOMAIN" "http://$SERVER_IP/release-marker.txt"
curl -fsS -H "Host: $PUBLIC_DOMAIN" "http://$SERVER_IP/api/status"
systemctl show prox-hermes-adapter -p FragmentPath -p EnvironmentFiles -p User -p NRestarts --no-pager
sudo bash scripts/deploy/check-adapter-health.sh
docker exec "${active_container:-new-api}" sh -c 'wget -qO- http://host.docker.internal:18181/health'
```

Expected ownership is `/opt/prox/current` for release source,
`/opt/prox/venv` for the Adapter environment, `/etc/prox/hermes.env` for
Adapter secrets, and `/var/lib/prox-hermes` for Adapter state. A host-side
`web/default/dist` change has no production effect until a tagged image is
built and a verified candidate becomes the explicit Nginx upstream after a
validated hot reload. The old container remains available until the retired
Nginx workers finish their existing requests.

## Backups

The backup transaction contains a PostgreSQL custom dump, Adapter state,
deployment configuration, secrets, release metadata, checksums, and a manifest.
The plaintext staging directory is removed by an exit trap and only the
authenticated age archive is retained.

Create a backup and run an isolated restore proof manually:

```bash
sudo bash scripts/deploy/backup.sh
sudo bash scripts/deploy/restore-drill.sh
sudo journalctl -u prox-backup -u prox-restore-drill --since today --no-pager
```

Configure either `BACKUP_AGE_RECIPIENT` or
`BACKUP_AGE_RECIPIENTS_FILE`. Keep `BACKUP_AGE_IDENTITY_FILE` in the operator
secret store and on the restore-drill host with mode `0600`. Set
`BACKUP_RCLONE_DEST` to copy each completed encrypted archive off-host;
`BACKUP_RCLONE_PRUNE=1` applies the same retention only inside that dedicated
destination. A restore drill decrypts into a private temporary directory,
checks every artifact hash, rejects unexpected archive paths, starts an
unpublished PostgreSQL 15 container with `--network none`, restores the dump,
and compares the public table count from the manifest.

## Disk retention

Inspect before cleanup:

```bash
df -h /
docker system df -v
docker buildx du
du -xhd1 /opt/prox /var/lib/docker 2>/dev/null | sort -h
```

Retain the active and previous `new-api` images. Remove only images not referenced by a container or `releases/current.env`. Compose bounds each container's JSON log to five 50 MiB files. `/etc/logrotate.d/prox` rotates host application and proxy logs daily with 14 compressed generations; validate it with `sudo logrotate --debug /etc/logrotate.d/prox`. Do not run broad prune commands during a release.

Use the repository cleanup policy instead of a broad Docker prune:

```bash
sudo bash scripts/deploy/cleanup.sh --dry-run
sudo bash scripts/deploy/cleanup.sh --apply
```

It considers only `IMAGE_REPOSITORY` release images and release metadata older
than `RELEASE_RETENTION_DAYS`. Images referenced by any container, the active
image, and the recorded rollback image are always retained. With
`BUILD_CACHE_PRUNE=1`, it also asks Docker to remove dangling build cache older
than the same retention period; it does not prune containers, volumes, or
networks.

## Operations timers

Install and enable all production operations units:

```bash
sudo install -m 600 deploy/systemd/prox-operations.env.example /etc/prox/operations.env
sudo cp deploy/systemd/prox-{monitor,backup,restore-drill,cleanup}.{service,timer} /etc/systemd/system/
sudo cp deploy/logrotate/prox /etc/logrotate.d/prox
sudo systemctl daemon-reload
sudo systemctl enable --now prox-monitor.timer prox-backup.timer prox-restore-drill.timer prox-cleanup.timer
systemctl list-timers 'prox-*' --no-pager
```

The schedules are one-minute monitoring, daily encrypted backup, monthly
restore proof, and weekly release cleanup. `Persistent=true` catches up a run
missed while the host was offline.

## Budget and game policy

Daily budget values, reset modes, check-in, invitation rewards, quiz rewards, and other game limits are configured in the operations console. Adapter-side plugin defaults are disabled in production. After changes, verify the effective group config and the budget ledger rather than only the form value.

## Quiz operations

1. Create or import a bank in Operations > Quiz banks.
2. Publish questions, then publish the bank.
3. Bind the bank to an exact platform/group or a wildcard fallback.
4. Verify one draw and one answer through the target chat platform.
5. Check draw, round, entry, and reward-ledger records.

## Key and membership recovery

Use the user/risk operations screens so restoration produces an audit trail. A key controlled by risk activation must restore both key status and its owning control; permission-only controls must be released without inventing an activation record.
