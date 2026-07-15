# Deployment

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

Fill `.env.deploy`, then install Adapter:

```bash
sudo python3 -m venv /opt/prox/venv
sudo /opt/prox/venv/bin/pip install \
  -r integrations/newapi-hermes-adapter/requirements.txt
sudo cp integrations/newapi-hermes-adapter/.env.example /etc/prox/hermes.env
sudo chmod 600 /etc/prox/hermes.env
sudo cp deploy/systemd/prox-hermes-adapter.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now prox-hermes-adapter
```

Start stateful services and release the application image:

```bash
docker compose --env-file .env.deploy -f compose.prod.yml up -d postgres redis oauth-worker new-api-proxy
bash scripts/deploy/preflight.sh
bash scripts/deploy/release.sh
```

For the first release, set `SKIP_ADAPTER_CHECK=1` only while bootstrapping Adapter health.

## Required proof

```bash
docker inspect new-api --format '{{.Config.Image}} {{.Image}} {{.State.Health.Status}} {{.RestartCount}}'
curl -fsS http://127.0.0.1:3000/api/status
curl -fsS http://127.0.0.1:3000/release-marker.txt
curl -fsS http://127.0.0.1:18181/health
```

The image tag and marker must match the release metadata under `releases/current.env`.

## Firewall

Expose only 80/443 publicly. Bind new-api and Adapter to loopback. Restrict SSH by source and key authentication. PostgreSQL and Redis remain on the Compose network.
