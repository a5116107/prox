"""Leaderboard plugin — tracks and displays player rankings."""

import json, os, time, threading
from game_plugins.base import GamePlugin, GameResponse

_STATE_DIR = os.environ.get("HERMES_STATE_DIR") or os.path.join(
    os.path.dirname(os.path.abspath(__file__)), ".."
)
STATS_FILE = os.environ.get("HERMES_LEADERBOARD_STORE") or os.path.join(
    _STATE_DIR, "leaderboard_stats.json"
)


class LeaderboardGame(GamePlugin):
    name = "leaderboard"
    display_name = "排行榜"
    description = "查看游戏排行榜"
    triggers = ["排行榜", "排名", "leaderboard", "rank", "龙虎榜"]
    group_required = False

    def __init__(self):
        super().__init__()
        self._lock = threading.Lock()
        self._stats = {}
        self._load()

    def _load(self):
        try:
            if os.path.exists(STATS_FILE):
                with open(STATS_FILE) as f:
                    self._stats = json.load(f)
        except Exception:
            self._stats = {}

    def _save(self):
        try:
            os.makedirs(os.path.dirname(os.path.abspath(STATS_FILE)), exist_ok=True)
            temp_path = STATS_FILE + ".tmp"
            with open(temp_path, "w") as f:
                json.dump(self._stats, f, ensure_ascii=False, indent=2)
            os.replace(temp_path, STATS_FILE)
        except Exception:
            pass

    def record(self, user_id, username, game_name, won=False, earned=0, spent=0):
        with self._lock:
            key = str(user_id)
            if key not in self._stats:
                self._stats[key] = {
                    "username": username,
                    "games_played": 0,
                    "wins": 0,
                    "total_earned": 0,
                    "total_spent": 0,
                    "last_active": 0,
                    "games": {},
                }
            s = self._stats[key]
            s["username"] = username or s["username"]
            s["games_played"] += 1
            if won:
                s["wins"] += 1
            s["total_earned"] += earned
            s["total_spent"] += spent
            s["last_active"] = int(time.time())
            s["games"][game_name] = s["games"].get(game_name, 0) + 1
            self._save()

    def handle(self, ctx, sm=None, budget=None, escrow=None):
        text = ctx.text.strip()

        parts = text.split()
        board_type = "games_played"
        title = "游戏次数"
        if len(parts) > 1:
            sub = parts[1]
            if sub in ["胜率", "胜场", "wins"]:
                board_type = "wins"
                title = "胜场数"
            elif sub in ["收益", "赚取", "earned"]:
                board_type = "total_earned"
                title = "总收益"
            elif sub in ["活跃", "active"]:
                board_type = "last_active"
                title = "最近活跃"

        with self._lock:
            stats = dict(self._stats)

        if not stats:
            return GameResponse.quick("暂无排行数据，快来玩游戏上榜吧！")

        sorted_users = sorted(
            stats.items(), key=lambda x: x[1].get(board_type, 0), reverse=True
        )[:10]

        lines = [f"\U0001f3c6 排行榜 — {title} TOP 10", ""]
        medals = ["\U0001f947", "\U0001f948", "\U0001f949"]
        for i, (uid, s) in enumerate(sorted_users):
            prefix = medals[i] if i < 3 else f" {i + 1}."
            name = s.get("username", uid)[:12]
            val = s.get(board_type, 0)
            if board_type == "last_active":
                if val > 0:
                    mins_ago = max(0, int((time.time() - val) / 60))
                    if mins_ago < 60:
                        val_str = f"{mins_ago}分钟前"
                    else:
                        val_str = f"{mins_ago // 60}小时前"
                else:
                    val_str = "-"
            elif board_type == "total_earned":
                val_str = f"${val / 500000:.2f}"
            else:
                val_str = str(val)
            extra = (
                f" ({s.get('wins', 0)}胜/{s.get('games_played', 0)}场)"
                if board_type != "games_played"
                else f" ({s.get('wins', 0)}胜)"
            )
            lines.append(f"{prefix} {name}: {val_str}{extra}")

        lines.append("")
        lines.append("\U0001f4a1 子命令: 排行榜 胜率 | 排行榜 收益 | 排行榜 活跃")
        return GameResponse.quick("\n".join(lines))
