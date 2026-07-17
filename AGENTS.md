# Prox Repository Guide

This repository owns the prox site only. Do not add configuration, domains,
deployment notes, or conditional behavior for separately maintained sites.

## Architecture

- Go/Gin API in `router/`, `controller/`, `service/`, and `model/`.
- React frontends in `web/default` and `web/classic`.
- PostgreSQL owns durable business data; Redis owns shared transient state.
- The Hermes Adapter in `integrations/newapi-hermes-adapter` normalizes QQ/TG
  events. New API remains the owner of permissions, rewards, quiz answers,
  memberships, and balances.
- Production topology and data ownership are documented in
  `docs/architecture.md`.

## Development Rules

- Preserve SQLite, MySQL, and PostgreSQL compatibility unless a package is
  explicitly PostgreSQL-only and documents that boundary.
- Use the JSON wrappers in `common/json.go` in Go business code.
- Use Bun for frontend dependency and script execution.
- Keep platform support generic. Select QQ/TG behavior from runtime
  configuration, never from another site's name or domain.
- Keep secrets, database files, TLS material, logs, caches, release archives,
  and host-specific addresses out of Git.
- Do not edit generated frontend assets directly. Build them from source.

## Verification

Run focused tests while changing code, then complete these gates before a
release:

```bash
bash scripts/test/prepare-embed-assets.sh
go test ./... -count=1 -timeout 15m

cd web/default
bun run typecheck
bun run test
bun run lint
bun run format:check
bun run i18n:check

cd ../classic
bun run lint
bun run eslint
bun run build

cd ../../integrations/newapi-hermes-adapter
ruff format --check ../../scripts/quiz/import_seed.py .
python -m pytest
```

Also validate both Compose files and all changed shell scripts. Quiz changes
must include the PostgreSQL concurrency test because locking behavior differs
from SQLite.

## Production Release Truth

The `new-api` Docker image is the release unit for both backend code and
embedded frontend assets. Host-side Go files and `web/*/dist` directories are
not mounted into the running container.

A production change is live only after a tagged image is built and a verified
`NODE_TYPE=slave` candidate has replaced the active traffic container. Load the
runtime paths before release:

```bash
set -a
source /etc/prox/operations.env
set +a
bash scripts/deploy/preflight.sh
bash scripts/deploy/release.sh
```

The release script starts and verifies the slave candidate before traffic,
renders that exact container name into the bind-mounted Nginx upstream, hot
reloads Nginx, waits for the retired Nginx workers to drain, and only then
stops the previous container and replaces the single `NODE_TYPE=master`
worker. It verifies `/api/status`, `/release-marker.txt`,
static assets, the quiz route, image configuration, and Adapter health, and
restores the prior traffic/worker pair on failure. Use
`scripts/deploy/rollback.sh` for an explicit candidate-first rollback.

Never use `docker compose down -v` for an application release. Never replace
or delete PostgreSQL data, Redis data, TLS material, `.env.deploy`, Adapter
state, or logs as part of source deployment.

## Documentation

- `README.md`: repository entry point and quick start.
- `docs/development.md`: local workflow and quiz seed import.
- `docs/deployment.md`: initial host installation.
- `docs/hot-update-and-rollback.md`: release and rollback procedure.
- `docs/operations.md`: routine checks and backup expectations.
- `docs/migration.md`: migration to another host.
- `docs/troubleshooting.md`: symptom-based diagnosis.

Update these documents with the code whenever a runtime contract changes.
