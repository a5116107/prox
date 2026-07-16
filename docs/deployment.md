# Deployment

## Release source

Production releases are built only from a clean checkout of
`https://github.com/a5116107/prox.git`. The production host does not own a Git
upstream and must not receive commits directly. `/opt/prox/current` is the
release checkout; the immutable `new-api` image is the deployed backend and
frontend unit.

## Host layout

```text
/opt/prox/current                  clean checkout of main
/opt/prox/venv                     Hermes Python environment
/opt/prox/current/.env.deploy      production secrets, mode 0600
/opt/prox/current/postgres-data    PostgreSQL data
/opt/prox/current/data             new-api data
/opt/prox/current/logs             new-api logs
/var/lib/prox-hermes               Adapter writable state
/etc/prox/hermes.env               Adapter secrets, mode 0600
```

## Initial installation

```bash
sudo useradd --system --home /opt/prox --shell /usr/sbin/nologin prox || true
sudo mkdir -p /opt/prox /etc/prox
sudo chown -R prox:prox /opt/prox
sudo -u prox git clone https://github.com/a5116107/prox.git /opt/prox/current
cd /opt/prox/current
cp .env.deploy.example .env.deploy
sudo chmod 600 .env.deploy
```

Fill `.env.deploy`, create the production Compose network, then install Adapter:

```bash
docker compose --env-file .env.deploy -f compose.prod.yml up -d postgres redis oauth-worker
sudo python3 -m venv /opt/prox/venv
sudo /opt/prox/venv/bin/pip install \
  -r integrations/newapi-hermes-adapter/requirements.txt
sudo cp integrations/newapi-hermes-adapter/.env.example /etc/prox/hermes.env
adapter_gateway="$(docker network inspect prox-prod_new-api-network \
  --format '{{(index .IPAM.Config 0).Gateway}}')"
sudo sed -i "s/^HERMES_ADAPTER_HOST=.*/HERMES_ADAPTER_HOST=$adapter_gateway/" /etc/prox/hermes.env
sudo chmod 600 /etc/prox/hermes.env
sudo cp deploy/systemd/prox-hermes-adapter.service /etc/systemd/system/
sudo systemctl daemon-reload
```

Set `HERMES_ADAPTER_KEY`/`GAME_ADMIN_KEY` to the same value as
`HERMES_SHARED_KEY` in `.env.deploy`. Set `CHATOPS_WEBHOOK_SECRET` to the same
secret saved under **Operations > Agent Ops**. Keep
`IMAGE_CONFIG_FROM_NEWAPI=true`, `IMAGE_CONFIG_CACHE_TTL_SECONDS=15`, and
`IMAGE_CONFIG_FETCH_TIMEOUT_SECONDS=2`.
`IMAGE_API_BASE_URL`, `IMAGE_API_KEY`, and the other `IMAGE_*` values are the
startup and New API outage fallback. A non-empty image key saved in Agent Ops
takes precedence and is never returned by the normal admin option API.

The fallback image key must live only in the mode-`0600` environment file.
Validate a configured fallback provider without printing the key, then start
the unit:

```bash
sudo bash -c 'set -a; source /etc/prox/hermes.env; set +a; \
  curl -fsS -H "Authorization: Bearer $IMAGE_API_KEY" \
  "$IMAGE_API_BASE_URL/models" | grep -q "gpt-image-2"'
sudo systemctl enable --now prox-hermes-adapter
sudo bash scripts/deploy/check-adapter-health.sh
```

Image settings saved in Agent Ops are read through the internal no-store
endpoint and become effective after at most `IMAGE_CONFIG_CACHE_TTL_SECONDS`;
the Adapter does not need a restart. Verify the endpoint and Adapter-selected
source without printing the image key:

```bash
set -a; source /etc/prox/hermes.env; set +a
curl -fsS -H "Authorization: Bearer $CHATOPS_WEBHOOK_SECRET" \
  'http://127.0.0.1:3000/api/agent/chatops/image-config?source=qq' \
  | jq -e '.success == true and (.data.api_key_configured | type == "boolean")' >/dev/null
sleep "${IMAGE_CONFIG_CACHE_TTL_SECONDS:-15}"
adapter_health_url="${HERMES_ADAPTER_HEALTH_URL:-http://${HERMES_ADAPTER_HOST}:${HERMES_ADAPTER_PORT:-18181}/health}"
curl -fsS "$adapter_health_url" \
  | jq -e '.ok == true and .image.source == "newapi"' >/dev/null
```

Release the application image:

```bash
bash scripts/deploy/preflight.sh
bash scripts/deploy/release.sh
docker compose --env-file .env.deploy -f compose.prod.yml up -d --no-deps new-api-proxy
```

For the first release, set `SKIP_ADAPTER_CHECK=1` only while bootstrapping Adapter health.

## Required proof

```bash
docker inspect new-api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}} {{.RestartCount}}'
git -C /opt/prox/current remote -v
git -C /opt/prox/current status --short --branch
git -C /opt/prox/current rev-parse HEAD
curl -fsS http://127.0.0.1:3000/api/status
curl -fsS http://127.0.0.1:3000/release-marker.txt
sudo bash scripts/deploy/check-adapter-health.sh
systemctl show prox-hermes-adapter -p FragmentPath -p EnvironmentFiles -p User -p NRestarts --no-pager
sudo bash -c '
  set -a; source /etc/prox/hermes.env; set +a
  health_url="${HERMES_ADAPTER_HEALTH_URL:-http://${HERMES_ADAPTER_HOST}:${HERMES_ADAPTER_PORT:-18181}/health}"
  curl -fsS "$health_url" | jq -e '\''.image.source == "newapi"'\'' >/dev/null
'
```

The checkout must list only GitHub as `origin`. The image tag, image ID, commit,
and marker must match the release metadata under `releases/current.env`.
Adapter `FragmentPath` must resolve to `prox-hermes-adapter.service`, its
environment file to `/etc/prox/hermes.env`, its user to `prox`, and its restart
count must remain stable.

## Firewall

Expose only 80/443 publicly. Bind new-api to loopback and Adapter to the
`prox-prod` Compose bridge gateway recorded in `/etc/prox/hermes.env`; do not
publish port 18181. Restrict SSH by source and key authentication. PostgreSQL
and Redis remain on the Compose network.
