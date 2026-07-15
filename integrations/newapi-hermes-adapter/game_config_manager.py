"""New API backed group game config resolver.

Primary truth: /api/agent/chatops/config/export, which is generated from
chat_groups, group_chatops_configs and group_game_configs in New API.
Local adapter game_config.json is no longer used for gameplay switches.
"""

from __future__ import annotations

import copy
import json
import os
import tempfile
import time
import urllib.parse
import urllib.request
from typing import Any, Dict


def _norm_platform(value: str) -> str:
    v = str(value or "").strip().lower()
    if v == "telegram":
        return "tg"
    return v or "unknown"


_SKIP_VALUE = object()


def _platform_candidates(value: str):
    p = _norm_platform(value)
    if p in ("qq", "qq_group"):
        return ["qq", "qq_group"]
    if p in ("tg", "tg_group"):
        return ["tg", "tg_group"]
    return [p]


def _clean_config_value(value: Any):
    if value is None:
        return _SKIP_VALUE
    if isinstance(value, str) and value.strip().lower() in {
        "<nil>",
        "nil",
        "null",
        "none",
    }:
        return _SKIP_VALUE
    if isinstance(value, dict):
        out = {}
        for key, item in value.items():
            cleaned = _clean_config_value(item)
            if cleaned is not _SKIP_VALUE:
                out[key] = cleaned
        return out
    if isinstance(value, list):
        out = []
        for item in value:
            cleaned = _clean_config_value(item)
            if cleaned is not _SKIP_VALUE:
                out.append(cleaned)
        return out
    return value


def _deep_merge(base: Dict[str, Any], extra: Dict[str, Any]) -> Dict[str, Any]:
    out = dict(base or {})
    for key, value in (extra or {}).items():
        value = _clean_config_value(value)
        if value is _SKIP_VALUE:
            continue
        if isinstance(value, dict) and isinstance(out.get(key), dict):
            out[key] = _deep_merge(out[key], value)
        else:
            out[key] = value
    return out


class GameConfigManager:
    def __init__(self):
        self.site_id = os.environ.get("SITE_ID", "default")
        self.base_url = (
            os.environ.get("NEWAPI_INTERNAL_BASE_URL")
            or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
            or os.environ.get("PUBLIC_BASE_URL")
            or "http://127.0.0.1:3000"
        ).rstrip("/")
        self.secret = (
            os.environ.get("CHATOPS_WEBHOOK_SECRET")
            or os.environ.get("NEWAPI_CHATOPS_SECRET")
            or os.environ.get("QQ_BRIDGE_SECRET")
            or ""
        ).strip()
        self.ttl = max(
            5, int(os.environ.get("HERMES_GAME_CONFIG_TTL_SECONDS", "30") or "30")
        )
        safe_site = "".join(
            ch if ch.isalnum() or ch in ("-", "_") else "_" for ch in self.site_id
        )
        self._config_path = os.environ.get("HERMES_GAME_CONFIG_CACHE") or os.path.join(
            tempfile.gettempdir(), f"newapi_group_game_config_{safe_site}.json"
        )
        self._last_load = 0.0
        self._config: Dict[str, Any] = {}
        self._source = "empty"
        self._load_initial()

    def _load_initial(self):
        if not self._reload_if_changed(force=True):
            cached = self._read_json(self._config_path)
            if cached:
                self._config = cached
                self._source = "cache"

    @staticmethod
    def _read_json(path: str) -> Dict[str, Any]:
        try:
            with open(path, "r", encoding="utf-8") as fh:
                data = json.load(fh)
            return data if isinstance(data, dict) else {}
        except Exception:
            return {}

    def _write_cache(self, data: Dict[str, Any]) -> None:
        try:
            os.makedirs(os.path.dirname(self._config_path), exist_ok=True)
            tmp = self._config_path + ".tmp"
            with open(tmp, "w", encoding="utf-8") as fh:
                json.dump(data, fh, ensure_ascii=False, indent=2)
            os.replace(tmp, self._config_path)
        except Exception as exc:
            print(f"[GameConfig] cache write failed: {exc}", flush=True)

    def _fetch_export(self) -> Dict[str, Any]:
        if not self.base_url or not self.secret:
            return {}
        query = urllib.parse.urlencode(
            {"site_id": self.site_id, "source": "admin", "secret": self.secret}
        )
        req = urllib.request.Request(
            f"{self.base_url}/api/agent/chatops/config/export?{query}",
            headers={
                "Authorization": "Bearer " + self.secret,
                "Accept": "application/json",
            },
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            body = json.loads(resp.read().decode("utf-8") or "{}")
        data = body.get("data") if isinstance(body.get("data"), dict) else body
        snapshot = data.get("snapshot") if isinstance(data, dict) else None
        if isinstance(snapshot, dict):
            return snapshot
        return {}

    def _reload_if_changed(self, force: bool = False) -> bool:
        now = time.time()
        if not force and now - self._last_load < self.ttl:
            return False
        self._last_load = now
        try:
            snapshot = self._fetch_export()
            if not snapshot:
                return False
            old = json.dumps(self._config, sort_keys=True, ensure_ascii=False)
            new = json.dumps(snapshot, sort_keys=True, ensure_ascii=False)
            self._config = snapshot
            self._source = "newapi_db_export"
            self._write_cache(snapshot)
            return old != new
        except Exception as exc:
            print(f"[GameConfig] export fetch failed: {exc}", flush=True)
            return False

    def _site(self) -> Dict[str, Any]:
        sites = (
            self._config.get("sites")
            if isinstance(self._config.get("sites"), dict)
            else {}
        )
        site = sites.get(self.site_id) or sites.get(str(self.site_id)) or {}
        return site if isinstance(site, dict) else {}

    def _platform(self, platform: str) -> Dict[str, Any]:
        plats = self._site().get("platforms")
        if not isinstance(plats, dict):
            return {}
        return (
            plats.get(_norm_platform(platform)) or plats.get(str(platform or "")) or {}
        )

    def _effective_game_config(self, ctx, game_code: str) -> Dict[str, Any]:
        platform = _norm_platform(
            getattr(ctx, "platform", "") or os.environ.get("BOT_PLATFORM", "")
        )
        group_id = str(getattr(ctx, "group_id", "") or "")
        merged: Dict[str, Any] = {}
        platforms = [self._platform(p) for p in _platform_candidates(platform)]
        for plat in platforms:
            platform_games = (
                plat.get("games") if isinstance(plat.get("games"), dict) else {}
            )
            if isinstance(platform_games.get(game_code), dict):
                merged = _deep_merge(merged, platform_games.get(game_code))
        for plat in platforms:
            groups = plat.get("groups") if isinstance(plat.get("groups"), dict) else {}
            group = groups.get(group_id) if group_id else {}
            if isinstance(group, dict):
                group_games = (
                    group.get("games") if isinstance(group.get("games"), dict) else {}
                )
                if isinstance(group_games.get(game_code), dict):
                    merged = _deep_merge(merged, group_games.get(game_code))
        return merged

    def is_game_enabled(self, ctx, game_code: str) -> bool:
        self._reload_if_changed()
        cfg = self._effective_game_config(ctx, game_code)
        if "enabled" in cfg:
            return bool(cfg.get("enabled"))
        # No explicit DB config means not open; avoids silently opening games
        # from adapter-local defaults when the control plane is empty.
        return os.environ.get("HERMES_GAME_ALLOW_PLUGIN_DEFAULTS", "").lower() in (
            "1",
            "true",
            "yes",
            "on",
        )

    def apply_game_config(self, ctx, plugin) -> Dict[str, Any]:
        self._reload_if_changed()
        game_code = str(getattr(plugin, "name", "") or "")
        defaults = copy.deepcopy(getattr(plugin, "default_config", {}) or {})
        cfg = self._effective_game_config(ctx, game_code)
        if not cfg and os.environ.get(
            "HERMES_GAME_ALLOW_PLUGIN_DEFAULTS", ""
        ).lower() not in ("1", "true", "yes", "on"):
            cfg = {"enabled": False}
        plugin.config = _deep_merge(defaults, cfg)
        return plugin.config


_manager = None


def get_config_manager() -> GameConfigManager:
    global _manager
    if _manager is None:
        _manager = GameConfigManager()
    return _manager
