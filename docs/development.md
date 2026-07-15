# Development

## Toolchain

- Go version from `go.mod`
- Bun `1.3.5`
- Python `3.11`
- PostgreSQL `15`, Redis `7`, Docker Compose v2

## Setup

```bash
git clone https://github.com/a5116107/prox.git
cd prox
cp .env.example .env
docker compose up -d postgres redis
```

Run the Go API directly with `go run .`, or build the complete development image with `docker compose up -d --build`.

For Adapter development:

```bash
python -m venv .venv
. .venv/bin/activate
pip install -r integrations/newapi-hermes-adapter/requirements.txt \
  -r integrations/newapi-hermes-adapter/requirements-dev.txt
ruff format --check scripts/quiz/import_seed.py integrations/newapi-hermes-adapter
python -m pytest integrations/newapi-hermes-adapter/tests -q
python integrations/newapi-hermes-adapter/adapter.py
```

## Change workflow

1. Branch from current `main`; use one worktree per agent or task.
2. Keep runtime data and credentials outside Git.
3. Add focused tests with the change, then run the affected package tests.
4. Run the full quality commands before merge.
5. Merge to a clean `main`; production release scripts reject tracked dirty files.

For a full Go test before building either frontend, create the ignored embed placeholders first:

```bash
bash scripts/test/prepare-embed-assets.sh
go test ./... -count=1
```

## Frontend

```bash
cd web
bun install --frozen-lockfile
cd default
bun run typecheck
bun run lint
bun run format:check
node scripts/sync-i18n.mjs check --json
bun run build
```

Validate `web/classic` with `bun run lint`, `bun run eslint`, and `bun run build`.
Host build output is only local evidence; production changes require a new Docker image.

## Quiz bank

The backend is authoritative. The Adapter calls `quiz.question.draw`, `quiz.round.load`, and `quiz.answer.submit`; it contains no rotating hard-coded list.

```bash
export PROX_ADMIN_TOKEN='root access token'
python scripts/quiz/import_seed.py --base-url http://127.0.0.1:3000
python scripts/quiz/import_seed.py --base-url http://127.0.0.1:3000 --apply --publish
```

The first call performs a read-only preview. The apply call repeats server dry-run before the transaction.

## Database tests

```bash
export QUIZ_POSTGRES_TEST_DSN='host=127.0.0.1 port=5432 user=prox_test password=prox_test dbname=prox_test sslmode=disable'
go test ./service -run '^TestQuizDrawConcurrentPostgres$' -count=1
```

The test creates and removes an isolated PostgreSQL schema.
