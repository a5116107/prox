"""Tiny adapter admin API used by New API's game-admin proxy."""

from __future__ import annotations

import json

ADMIN_PATH_PREFIX = "/game-admin"
ADMIN_HTML = """<!doctype html><meta charset="utf-8"><title>Game Admin</title>
<body style="font-family:system-ui;padding:24px">
<h1>Game Admin</h1>
<p>玩法与群组配置主入口已迁移到 New API 后台 Ops 控制平面；本页仅保留适配器健康检查。</p>
</body>"""


class GameAdminHandler:
    @staticmethod
    def handle(path, payload, method="GET"):
        try:
            from game_director_integration import get_director

            director = get_director()
            games = []
            for code, plugin in sorted((director.director.plugins or {}).items()):
                games.append(
                    {
                        "code": code,
                        "name": getattr(plugin, "display_name", code),
                        "enabled": bool((plugin.config or {}).get("enabled", True)),
                        "config": plugin.config,
                    }
                )
            return 200, {
                "ok": True,
                "source": "adapter",
                "message": "玩法配置主真值在 New API Ops 控制平面。",
                "games": games,
            }
        except Exception as exc:
            return 500, {"ok": False, "error": str(exc)}
