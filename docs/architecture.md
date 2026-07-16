# Architecture

## Runtime components

```text
Client / QQ / TG
       |
       v
Nginx proxy ----> new-api image (Go API + embedded Default/Classic assets)
                       |                 |
                       v                 v
                 PostgreSQL           Redis
                       ^
                       |
Hermes Adapter <-------+---- internal chatops/action APIs
       |
       +---- OneBot / Telegram / model endpoint
```

`new-api` owns business state and authorization. Hermes normalizes platform events and renders replies; it does not own balances, rewards, memberships, quiz answers, or group permissions.

## Data ownership

| Data | Owner | Persistence |
| --- | --- | --- |
| Users, keys, channels, balances | new-api | PostgreSQL |
| Community membership and risk controls | new-api | PostgreSQL |
| Group settings and game policy | new-api | PostgreSQL, exported to Adapter cache |
| Quiz banks, draws, answers, rewards | new-api | PostgreSQL |
| Asynchronous task terminal state and billing outbox | new-api | PostgreSQL |
| Rate-limit and transient cache | new-api | Redis |
| Adapter game state, learning, leaderboard | Hermes Adapter | `HERMES_STATE_DIR` |

## Consistency rules

1. Community Bot messages are claimed with a lease/fence before externally visible work is committed; cursors only advance.
2. Reward and quota mutations use database transactions plus idempotency keys.
3. Risk activation codes are returned to the UI immediately after key creation; controlled keys remain traceable and restorable.
4. Quiz draw selection locks the bank row and rechecks the active scope after lock acquisition. One user/group scope has one open round.
5. Adapter config defaults are development-only. Production receives group/game configuration from new-api.
6. A Task or Midjourney terminal transition and its `task_billing_operations`
   row commit in one PostgreSQL transaction. Wallet/subscription funding,
   token totals, aggregate usage, cache invalidation, and logs are replayed as
   idempotent outbox steps.
7. Every API node may run the task-billing worker. A fenced lease permits one
   node to process a row at a time; expired leases and partially completed rows
   are resumed. A separate `LOG_SQL_DSN` remains supported because billing logs
   carry a nullable unique idempotency key.

## Deployment boundary

The Docker image is the release unit for backend and frontend. `compose.prod.yml` mounts only data and logs into `new-api`. Hermes runs as a separate systemd service with state under `/var/lib/prox-hermes` and is reached from Docker through `host.docker.internal`.

## Scale path

PostgreSQL and Redis are already external to the application image. Adding API replicas requires a stable reverse-proxy upstream, shared session secret, shared Redis/PostgreSQL, and one-time job/Community Bot leadership. The current release tooling deliberately switches one `new-api` service and never recreates stateful services.
