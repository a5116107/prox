"""Lightweight DB logger for activity plans and task runs. Writes directly to Postgres.

Postgres runs in a Docker container whose IP changes on every `docker compose
up -d` recreate, so we resolve the current IP via `docker inspect` at connect
time (env override DBLOGGER_PG_HOST wins; stale-IP fallback for safety).
"""

import os, time, json, threading, subprocess

_conn = None
_lock = threading.Lock()
_cached_host = None
_cached_container_env = {}


def _env_first(*names, default=""):
    for name in names:
        value = os.environ.get(name)
        if value:
            return value
    return default


def _container_candidates():
    candidates = []
    explicit = _env_first("DBLOGGER_PG_CONTAINER", "POSTGRES_CONTAINER_NAME")
    if explicit:
        candidates.append(explicit)
    for name in ("new-api-pg", "postgres", "db"):
        if name not in candidates:
            candidates.append(name)
    return candidates


def _inspect_container_env(container_name):
    cached = _cached_container_env.get(container_name)
    if cached is not None:
        return cached
    try:
        out = (
            subprocess.check_output(
                [
                    "docker",
                    "inspect",
                    "-f",
                    "{{range .Config.Env}}{{println .}}{{end}}",
                    container_name,
                ],
                timeout=3,
                stderr=subprocess.DEVNULL,
            )
            .decode()
            .splitlines()
        )
    except Exception:
        out = []
    env_map = {}
    for line in out:
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        env_map[key] = value
    _cached_container_env[container_name] = env_map
    return env_map


def _inspect_pg_value(*keys, default=""):
    for container_name in _container_candidates():
        env_map = _inspect_container_env(container_name)
        if not env_map:
            continue
        for key in keys:
            value = env_map.get(key)
            if value:
                return value
    return default


def _pg_host():
    """Resolve current postgres container IP. Env override wins."""
    global _cached_host
    env_host = _env_first("DBLOGGER_PG_HOST", "PGHOST")
    if env_host:
        return env_host
    for container_name in _container_candidates():
        try:
            out = (
                subprocess.check_output(
                    [
                        "docker",
                        "inspect",
                        "-f",
                        "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
                        container_name,
                    ],
                    timeout=3,
                    stderr=subprocess.DEVNULL,
                )
                .decode()
                .strip()
            )
            if out:
                _cached_host = out
                return out
        except Exception:
            continue
    return _cached_host or ""


def _pg_port():
    return int(_env_first("DBLOGGER_PG_PORT", "PGPORT", default="5432"))


def _pg_db():
    return _env_first(
        "DBLOGGER_PG_DB",
        "POSTGRES_DB",
        default=_inspect_pg_value("POSTGRES_DB", default="new-api"),
    )


def _pg_user():
    return _env_first(
        "DBLOGGER_PG_USER",
        "POSTGRES_USER",
        default=_inspect_pg_value("POSTGRES_USER", default="newapi"),
    )


def _pg_password():
    return _env_first(
        "DBLOGGER_PG_PASSWORD",
        "DBLOGGER_POSTGRES_PASSWORD",
        "POSTGRES_PASSWORD",
        default=_inspect_pg_value("POSTGRES_PASSWORD"),
    )


def _get_conn():
    global _conn
    if _conn is not None:
        try:
            _conn.cursor().execute("SELECT 1")
            return _conn
        except Exception:
            try:
                _conn.close()
            except Exception:
                pass
            _conn = None
    host = _pg_host()
    user = _pg_user()
    password = _pg_password()
    if not host or not user or not password:
        print(
            "[DBLogger] disabled: PostgreSQL host/user/password are not configured",
            flush=True,
        )
        return None
    try:
        import psycopg2

        _conn = psycopg2.connect(
            host=host,
            port=_pg_port(),
            dbname=_pg_db(),
            user=user,
            password=password,
            connect_timeout=4,
        )
        _conn.autocommit = True
        return _conn
    except Exception as e:
        print(f"[DBLogger] connect error (host={host}): {e}", flush=True)
        return None


def log_activity_plan(
    site_id,
    plan_type,
    title,
    status="completed",
    budget_quota=0,
    rules=None,
    result=None,
):
    with _lock:
        conn = _get_conn()
        if not conn:
            return None
        try:
            cur = conn.cursor()
            now = int(time.time())
            cur.execute(
                """INSERT INTO agent_activity_plans
                   (site_id, plan_type, title, status, budget_quota, start_at, end_at, rules_json, result_json, created_by, updated_at, created_at)
                   VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, 0, %s, %s)
                   RETURNING id""",
                (
                    site_id,
                    plan_type,
                    title,
                    status,
                    budget_quota,
                    now,
                    now,
                    json.dumps(rules or {}, ensure_ascii=False),
                    json.dumps(result or {}, ensure_ascii=False),
                    now,
                    now,
                ),
            )
            row = cur.fetchone()
            return row[0] if row else None
        except Exception as e:
            print(f"[DBLogger] activity_plan error: {e}", flush=True)
            # connection may be stale; force re-resolve next time
            try:
                _conn.close()
            except Exception:
                pass
            _conn = None
            return None


def log_task_run(
    site_id,
    task_id=0,
    run_type="scheduler",
    status="completed",
    input_data=None,
    output_data=None,
    error_msg="",
):
    with _lock:
        conn = _get_conn()
        if not conn:
            return None
        try:
            cur = conn.cursor()
            now = int(time.time())
            cur.execute(
                """INSERT INTO agent_task_runs
                   (site_id, task_id, action_id, run_type, status, input_json, output_json, error, started_at, finished_at, updated_at, created_at)
                   VALUES (%s, %s, 0, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                   RETURNING id""",
                (
                    site_id,
                    task_id,
                    run_type,
                    status,
                    json.dumps(input_data or {}, ensure_ascii=False),
                    json.dumps(output_data or {}, ensure_ascii=False),
                    error_msg,
                    now,
                    now,
                    now,
                    now,
                ),
            )
            row = cur.fetchone()
            return row[0] if row else None
        except Exception as e:
            print(f"[DBLogger] task_run error: {e}", flush=True)
            try:
                _conn.close()
            except Exception:
                pass
            _conn = None
            return None
