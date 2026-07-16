# Operations

## Daily checks

```bash
docker compose --env-file .env.deploy -f compose.prod.yml ps
docker inspect new-api --format '{{.Config.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}'
systemctl is-active prox-hermes-adapter
curl -fsS http://127.0.0.1:3000/api/status
sudo bash scripts/deploy/check-adapter-health.sh
df -h /
docker system df
```

Review Nginx status counts, API latency, upstream 429s, Bot action failures, budget pool balance, and risk-control audit events.

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
docker inspect new-api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}'
docker inspect new-api --format '{{json .Mounts}}'
curl -fsS http://127.0.0.1:3000/release-marker.txt
curl -fsS http://127.0.0.1:3000/api/status
systemctl show prox-hermes-adapter -p FragmentPath -p EnvironmentFiles -p User -p NRestarts --no-pager
sudo bash scripts/deploy/check-adapter-health.sh
docker exec new-api sh -c 'wget -qO- http://host.docker.internal:18181/health'
```

Expected ownership is `/opt/prox/current` for release source,
`/opt/prox/venv` for the Adapter environment, `/etc/prox/hermes.env` for
Adapter secrets, and `/var/lib/prox-hermes` for Adapter state. A host-side
`web/default/dist` change has no production effect until a tagged image is
built and `new-api` is recreated.

## Backups

- PostgreSQL: daily custom-format dump, encrypted off-host copy, retention test.
- `.env.deploy` and `/etc/prox/hermes.env`: encrypted secret store, not Git.
- `/var/lib/prox-hermes`: daily file backup.
- Restore rehearsal: at least monthly on an isolated database.

## Disk retention

Inspect before cleanup:

```bash
df -h /
docker system df -v
docker buildx du
du -xhd1 /opt/prox /var/lib/docker 2>/dev/null | sort -h
```

Retain the active and previous `new-api` images. Remove only images not referenced by a container or `releases/current.env`. Rotate application/proxy logs through host policy; do not run broad prune commands during a release.

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
