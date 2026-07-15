# prox

prox is the source of truth for the `prox` API site, its operations console, community automation, QQ/TG adapter, games, risk controls, and quota accounting. It is maintained independently from other sites.

## Repository layout

| Path | Responsibility |
| --- | --- |
| `controller/`, `service/`, `model/`, `router/` | Go API and business rules |
| `web/default/` | Primary operations and user console |
| `web/classic/` | Compatibility frontend |
| `integrations/newapi-hermes-adapter/` | QQ/TG chat and game adapter |
| `seed/` | Versioned, non-secret seed data |
| `scripts/deploy/` | Preflight, release, rollback, and env migration |
| `scripts/quiz/` | Quiz-bank import tooling |
| `deploy/systemd/` | Host service units |

## Local start

Requirements: Docker Engine with Compose v2.

```bash
cp .env.example .env
docker compose up -d --build
curl http://127.0.0.1:3000/api/status
curl http://127.0.0.1:18181/health
```

The development stack starts PostgreSQL, Redis, the embedded frontend/backend image, and Hermes Adapter. Runtime data is ignored by Git.

## Quality checks

```bash
bash scripts/test/prepare-embed-assets.sh
go test ./... -count=1
python -m pytest integrations/newapi-hermes-adapter/tests -q
cd web && bun install --frozen-lockfile
cd default && bun run typecheck && bun run lint && bun run i18n:check && bun run build
cd ../classic && bun run build
```

The prepare script only creates ignored placeholder assets when a clean checkout has not built either frontend. The real PostgreSQL quiz concurrency test is enabled with `QUIZ_POSTGRES_TEST_DSN`; CI provisions PostgreSQL automatically.

## Production release

Production serves the Go binary and both frontend builds from one immutable image. Editing host source or `web/default/dist` does not update the live container.

```bash
cp .env.deploy.example .env.deploy
# Fill all values, install the Hermes systemd unit, then:
bash scripts/deploy/preflight.sh
bash scripts/deploy/release.sh
```

The release command builds a unique image, switches only `new-api`, verifies health, the quiz route, and `/release-marker.txt`, then records rollback metadata. See [deployment](docs/deployment.md) and [hot update and rollback](docs/hot-update-and-rollback.md).

## Documentation

- [Architecture](docs/architecture.md)
- [Development](docs/development.md)
- [Deployment](docs/deployment.md)
- [Hot update and rollback](docs/hot-update-and-rollback.md)
- [Migration](docs/migration.md)
- [Operations](docs/operations.md)
- [Troubleshooting](docs/troubleshooting.md)

## Repository rules

- `main` is releasable and must pass `.github/workflows/quality.yml`.
- Production data, TLS keys, `.env.deploy`, logs, caches, and backups never enter Git.
- Schema changes are backward-compatible before a release switches the application image.
- A production result is proven from the active container image and live routes, not from host build output.
