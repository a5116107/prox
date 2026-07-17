# MM Portal And AWX Operations

## Scope and ownership

MM Portal and AWX are an operations plane. They are deployed independently
from the prox New API release and must not be added to the prox application
Compose project.

```text
Cloudflare (mm.acompany.cv)
             |
             v
target Nginx (new-api-proxy)
       |                    |
       v                    v
MM Portal container      K3s NodePort
                             |
                             v
                       AWX 24.6.1
```

The production inventory, addresses, SSH fingerprints, and secrets stay in the
operator secret store. Do not commit them to this repository. The old prox host
no longer runs MM Portal or K3s/AWX; changing DNS back without first restoring
those services is not a rollback.

## Target layout

| Component | Runtime or persistent path |
| --- | --- |
| MM Portal application | `/opt/apps/mm-ops-portal` |
| MM Portal active state | `/opt/apps/mm-ops-portal/runtime` |
| MM Portal retained rollback state | `/opt/apps/mm-ops-portal/.rollback-runtime-target-20260718T032900` |
| K3s state | `/var/lib/rancher/k3s` |
| AWX migration backup | `/opt/ops-backups/mm-awx-migration-20260718T033800` |
| Nginx configuration | `/opt/new-api-dev/proxy` |
| Docker state for all target-host services | `/var/lib/containerd` |

`/var/lib/containerd` is the active Docker data root on this host. It is not a
K3s cache and must not be deleted. K3s owns `/var/lib/rancher/k3s`.

## Daily health check

Run the checks on the target host:

```bash
systemctl is-active k3s
k3s kubectl get nodes -o wide
k3s kubectl get pods -A
docker inspect mm-ops-portal new-api-proxy \
  --format '{{.Name}} {{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}} restarts={{.RestartCount}}'

curl --noproxy '*' -kfsS --max-time 15 \
  --resolve mm.acompany.cv:443:127.0.0.1 \
  https://mm.acompany.cv/ -o /dev/null
curl --noproxy '*' -kfsS --max-time 15 \
  --resolve mm.acompany.cv:443:127.0.0.1 \
  https://mm.acompany.cv/awx/api/v2/ping/ | jq .

df -h /
du -sh /opt/apps/mm-ops-portal /var/lib/rancher/k3s /var/lib/containerd
docker system df
```

Also verify both public routes through Cloudflare. A direct origin check proves
the local proxy and upstream; a public check proves DNS, Cloudflare, TLS, and
the origin path together.

## Backup verification

The AWX backup directory contains a PostgreSQL custom dump, Kubernetes
resources, project data, and `SHA256SUMS`. Before relying on it:

```bash
cd /opt/ops-backups/mm-awx-migration-20260718T033800
sha256sum -c SHA256SUMS
pg_restore -l AWX_DUMP_FILE >/dev/null
gzip -t PROJECT_ARCHIVE_FILE
```

Use the actual names listed in `SHA256SUMS`. The MM rollback database is a
separate retained point. Verify it read-only:

```bash
python3 - <<'PY'
import sqlite3

path = "/opt/apps/mm-ops-portal/.rollback-runtime-target-20260718T032900/ops_portal.db"
db = sqlite3.connect("file:" + path + "?mode=ro", uri=True)
assert db.execute("PRAGMA quick_check").fetchone()[0] == "ok"
db.close()
PY
```

Copy both AWX and MM backups off-host. A backup that exists only beside the
running service does not cover host loss.

## MM rollback

The target retains one previous MM image and one migration-time runtime copy.
Perform a rollback only after recording the current database hash and stopping
the writer:

```bash
set -euo pipefail
cd /opt/apps/mm-ops-portal
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
docker stop mm-ops-portal
cp --reflink=auto --sparse=always runtime/ops_portal.db \
  "/opt/ops-backups/mm-awx-migration-20260718T033800/ops_portal.before-rollback.$stamp.db"
uid="$(stat -c %u runtime/ops_portal.db)"
gid="$(stat -c %g runtime/ops_portal.db)"
mode="$(stat -c %a runtime/ops_portal.db)"
cp --reflink=auto --sparse=always \
  .rollback-runtime-target-20260718T032900/ops_portal.db \
  runtime/.ops_portal.db.restore
chown "$uid:$gid" runtime/.ops_portal.db.restore
chmod "$mode" runtime/.ops_portal.db.restore
mv -f runtime/.ops_portal.db.restore runtime/ops_portal.db
docker start mm-ops-portal
curl --noproxy '*' -kfsS --max-time 15 \
  --resolve mm.acompany.cv:443:127.0.0.1 \
  https://mm.acompany.cv/ -o /dev/null
```

Switch the container image to the retained previous tag only when application
code rollback is also required. Database rollback discards writes made after
the retained point, so preserve the current database first.

## AWX restore outline

1. Put the MM automation integration into maintenance mode.
2. Verify the backup checksums and stop AWX web/task workloads.
3. Restore the custom PostgreSQL dump into a clean AWX PostgreSQL instance.
4. Restore project data and apply the saved Kubernetes resources.
5. Wait for PostgreSQL, operator, web, and task Pods to become Ready.
6. Verify `/awx/api/v2/ping/`, then compare organization, inventory, template,
   credential, project, and user counts with the backup receipt.
7. Re-enable MM automation only after a read-only AWX API request succeeds.

Do not restore a database dump over a running AWX writer.

## K3s disk retention

Containerd image imports may leave `extract-*` snapshots protected by leases
that carry `containerd.io/gc.expire`. They normally disappear after the lease
expires. Prefer waiting for expiry when disk pressure is low.

Before an accelerated cleanup, prove all of the following:

```bash
ps -eo pid,etimes,cmd | grep -E \
  'ctr .* (pull|import)|crictl pull|containerd.*unpack' | grep -v grep || true
k3s ctr -n k8s.io leases ls
k3s ctr -n k8s.io snapshots ls
k3s crictl ps -a
k3s kubectl get pods -A
```

Only short lease IDs carrying the GC-expiry label are candidates. Keep every
64-character Pod/container lease. Remove candidates in small batches, wait for
GC, and repeat the daily health check after each batch. Save the before lists
under the dated backup directory. Never remove snapshot directories directly.

## Retention policy

- Keep the active MM image and one previous MM image.
- Keep one verified MM runtime rollback point until an off-host backup and a
  restore drill both succeed.
- Keep the dated AWX migration backup until the replacement backup policy has
  produced and restored a newer archive.
- Remove duplicate `.bak` trees and stopped containers only after confirming
  they do not provide a distinct image or persistent-data point.
- Prune build cache outside releases; do not run broad image, volume, or
  container prune commands on this shared host.

## Migration acceptance record

The 2026-07-18 migration was accepted only after:

- Cloudflare probe requests appeared in the target Nginx access log and not on
  the retired source host;
- MM Portal and AWX public routes returned HTTP 200 repeatedly;
- the K3s node was Ready and all long-running AWX Pods were Running;
- AWX object counts and the MM database digest matched the source receipt;
- the old host no longer contained the MM Portal, AWX configuration, or K3s
  state; and
- both hosts retained healthy prox services after source cleanup.

The host-specific receipt and exact hashes live in the private operations
workspace and the dated target backup directory, not in Git.
