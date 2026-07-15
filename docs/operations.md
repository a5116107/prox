# Operations

## Daily checks

```bash
docker compose --env-file .env.deploy -f compose.prod.yml ps
docker inspect new-api --format '{{.Config.Image}} {{.State.Health.Status}} restarts={{.RestartCount}}'
systemctl is-active prox-hermes-adapter
curl -fsS http://127.0.0.1:3000/api/status
curl -fsS http://127.0.0.1:18181/health
df -h /
docker system df
```

Review Nginx status counts, API latency, upstream 429s, Bot action failures, budget pool balance, and risk-control audit events.

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
