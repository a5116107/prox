# Migration

## Source and Git history

The public repository contains a fresh `main` history. Old database files, certificates, environment files, caches, backups, and incident snapshots are intentionally excluded. Move runtime state through backup/restore, never through Git.

## Environment migration

```bash
bash scripts/deploy/migrate-env.sh /old/path/.env.deploy /opt/prox/current/.env.deploy.migrated
diff -u .env.deploy.example .env.deploy.migrated
mv .env.deploy.migrated .env.deploy
chmod 600 .env.deploy
```

Known values are merged into the current template. Unknown legacy keys are retained in a marked section for explicit review.

## PostgreSQL

On the source host:

```bash
docker exec new-api-pg pg_dump -U newapi -d new-api -Fc > prox.dump
sha256sum prox.dump > prox.dump.sha256
```

On the destination, stop writes, verify the checksum, then restore into an empty PostgreSQL 15 database:

```bash
sha256sum -c prox.dump.sha256
docker compose --env-file .env.deploy -f compose.prod.yml stop new-api
cat prox.dump | docker exec -i new-api-pg pg_restore -U newapi -d new-api --clean --if-exists
```

Start the selected image and run route-level smoke tests before DNS cutover.

## Redis and Adapter state

Redis contains transient/cache state; normally start it empty. Copy `/var/lib/prox-hermes` with ownership preserved for Adapter game/learning continuity:

```bash
rsync -aHAX --numeric-ids source:/var/lib/prox-hermes/ /var/lib/prox-hermes/
chown -R prox:prox /var/lib/prox-hermes
```

## Cutover

1. Lower DNS TTL before the window.
2. Restore and validate through the destination host directly.
3. Stop source writes and take the final database delta/full dump.
4. Switch DNS/proxy, monitor errors and first-token latency.
5. Keep the source read-only until the rollback window closes.
