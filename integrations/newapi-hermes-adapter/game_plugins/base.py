"""Game Plugin Base — with per-platform budget pools + configurable limits."""

import os, random, threading, time, json
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Tuple


def _load_pool_config():
    try:
        _cfg_path = os.path.join(
            os.path.dirname(os.path.abspath(__file__)), os.pardir, "game_config.json"
        )
        with open(_cfg_path) as _f:
            _cfg = json.load(_f)
        _ps = _cfg.get("pool_settings", {})
        _pl = _ps.get("pool_limits", _cfg.get("pool_limits", {}))
        _limits = {}
        for _k in ["game", "activity", "ops_comp", "community"]:
            try:
                _limits[_k] = int(_pl.get(_k, 10000000))
            except:
                _limits[_k] = 10000000
        _ul = int(_ps.get("user_daily_limit", 1500000))
        _qpu = int(_ps.get("quota_per_usd", 500000))
        return _limits, _ul, _qpu
    except Exception:
        pass
    return (
        {
            "game": 10000000,
            "activity": 10000000,
            "ops_comp": 5000000,
            "community": 10000000,
        },
        1500000,
        500000,
    )


POOL_LIMITS, USER_DAILY_LIMIT, QUOTA_PER_USD = _load_pool_config()
_last_pool_load = time.time()


def reload_pool_limits():
    global POOL_LIMITS, USER_DAILY_LIMIT, QUOTA_PER_USD, _last_pool_load
    POOL_LIMITS, USER_DAILY_LIMIT, QUOTA_PER_USD = _load_pool_config()
    _last_pool_load = time.time()


def format_card(title, rows=None, footer="", emoji=""):
    """Render a compact plain-text card usable across QQ/TG/community adapters.

    rows accepts either plain strings or (label, value) tuples. Kept dependency-free
    so utility plugins can import it from base without creating cross-plugin cycles.
    """
    rows = rows or []
    head = f"{emoji} {title}".strip()
    lines = [head, "━━━━━━━━━━━━"]
    for row in rows:
        if isinstance(row, (list, tuple)) and len(row) >= 2:
            label = str(row[0]).strip()
            value = str(row[1]).strip()
            if label and value:
                lines.append(f"{label}：{value}")
            elif value:
                lines.append(value)
        else:
            text = str(row).strip()
            if text:
                lines.append(text)
    if footer:
        lines.extend(["━━━━━━━━━━━━", str(footer).strip()])
    return "\n".join(lines)


@dataclass
class GameContext:
    site_id: str = ""
    site_name: str = ""
    platform: str = ""
    group_id: str = ""
    user_id: str = ""
    username: str = ""
    new_api_user_id: int = 0
    new_api_token: str = ""
    text: str = ""
    quota_balance: int = 0
    is_admin: bool = False
    is_bound: bool = False


@dataclass
class GameResponse:
    reply: str = ""
    actions: List[Dict] = field(default_factory=list)
    event: str = ""

    @staticmethod
    def quick(text, **kw):
        return GameResponse(reply=text, **kw)


@dataclass
class GameSession:
    game_name: str
    session_id: str
    group_id: str
    started_at: float
    expires_at: float
    state: Dict = field(default_factory=dict)
    participants: Dict = field(default_factory=dict)

    def is_expired(self):
        return time.time() > self.expires_at

    def time_left(self):
        return max(0, int(self.expires_at - time.time()))


class BudgetChecker:
    def __init__(self, base_url, token):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self._daily = {}
        self._user = {}
        self._income = {}
        self._platform = "default"
        self._income_events = []
        self._income_seq = 0
        self._qpu = QUOTA_PER_USD
        self._lock = threading.Lock()

    def set_platform(self, platform):
        self._platform = platform or "default"

    def _pk(self, pool, platform=None):
        return f"{platform or self._platform}:{pool}"

    def usd_to_quota(self, usd):
        return int(usd * self._qpu)

    def quota_to_usd(self, q):
        return round(q / self._qpu, 2)

    def check_budget(self, pool, amount, platform=None):
        # V2: 池限额由 Go AgentSetting 真实控制(ApplyQuotaMutationTx 检查);adapter 向后兼容,总通过
        return True, ""

    def check_user_limit(self, uid, amount, pool="game", platform=None):
        # The Go settlement transaction is authoritative for per-user and pool
        # limits. Local counters are process-scoped and remain reporting-only.
        return True, ""

    def deduct(self, uid, amount, pool="game", platform=None):
        pk = self._pk(pool, platform)
        with self._lock:
            self._daily[pk] = self._daily.get(pk, 0) + amount
            self._user.setdefault(uid, {})[pk] = (
                self._user.get(uid, {}).get(pk, 0) + amount
            )

    def record_income(self, amount, pool="game", platform=None):
        platform = platform or self._platform
        pk = self._pk(pool, platform)
        with self._lock:
            self._income[pk] = self._income.get(pk, 0) + amount
            self._income_seq += 1
            self._income_events.append(
                {
                    "seq": self._income_seq,
                    "amount": int(amount or 0),
                    "pool": pool or "game",
                    "platform": platform or "default",
                    "thread_id": threading.get_ident(),
                    "ts": time.time(),
                }
            )
            if len(self._income_events) > 500:
                self._income_events = self._income_events[-500:]

    def peek_income_events(self, pool=None, platform=None, thread_id=None, max_age=120):
        now = time.time()
        out = []
        with self._lock:
            # Drop stale uncommitted events; they belong to failed/abandoned settlements.
            self._income_events = [
                e
                for e in self._income_events
                if now - float(e.get("ts", now)) <= max_age
            ]
            for e in self._income_events:
                if pool is not None and e.get("pool") != pool:
                    continue
                if platform is not None and e.get("platform") != platform:
                    continue
                if thread_id is not None and int(e.get("thread_id", -1)) != int(
                    thread_id
                ):
                    continue
                out.append(dict(e))
        return out

    def commit_income_events(self, seqs):
        try:
            seqset = {int(x) for x in (seqs or [])}
        except Exception:
            seqset = set()
        if not seqset:
            return 0
        with self._lock:
            before = len(self._income_events)
            self._income_events = [
                e for e in self._income_events if int(e.get("seq", 0)) not in seqset
            ]
            return before - len(self._income_events)

    def get_status(self, platform=None):
        plat = platform or self._platform
        r = {}
        for pool, limit in POOL_LIMITS.items():
            pk = self._pk(pool, plat)
            s = self._daily.get(pk, 0)
            income = self._income.get(pk, 0)
            rem = max(limit - s, 0)
            r[pool] = {
                "limit": limit,
                "limit_usd": self.quota_to_usd(limit),
                "spent": s,
                "spent_usd": self.quota_to_usd(s),
                "income": income,
                "income_usd": self.quota_to_usd(income),
                "remaining": rem,
                "remaining_usd": self.quota_to_usd(rem),
                "usage_pct": round(s / limit * 100, 1) if limit > 0 else 0,
                "funding_model": "ops_fund",
                "note": "统计收入仅用于观察，真实可发额度由运营基金承担",
            }
        return r

    def get_admin_report(self, platform=None):
        """Full pool report for admin only — includes all internal numbers."""
        plat = platform or self._platform
        r = {}
        total_spent = 0
        total_income = 0
        for pool, limit in POOL_LIMITS.items():
            pk = self._pk(pool, plat)
            sp = self._daily.get(pk, 0)
            income = self._income.get(pk, 0)
            rem = max(limit - sp, 0)
            total_spent += sp
            total_income += income
            r[pool] = {
                "limit": limit,
                "limit_usd": self.quota_to_usd(limit),
                "spent": sp,
                "spent_usd": self.quota_to_usd(sp),
                "income": income,
                "income_usd": self.quota_to_usd(income),
                "net": income - sp,
                "net_usd": self.quota_to_usd(income - sp),
                "remaining": rem,
                "remaining_usd": self.quota_to_usd(rem),
                "usage_pct": round(sp / limit * 100, 1) if limit > 0 else 0,
                "funding_model": "ops_fund",
            }
        r["_summary"] = {
            "total_spent_usd": self.quota_to_usd(total_spent),
            "total_income_usd": self.quota_to_usd(total_income),
            "net_usd": self.quota_to_usd(total_income - total_spent),
            "active_users": len(self._user),
            "funding_note": "所有发放都由运营基金对冲，收入字段仅用于统计观察",
        }
        return r

    def reset_daily(self):
        with self._lock:
            self._daily.clear()
            self._user.clear()
            self._income.clear()


class SessionManager:
    def __init__(self):
        self._s = {}
        self._l = threading.Lock()

    def get_session(self, gid):
        """Return a session even when expired.

        Expiry handlers must inspect the original state before cleanup, while
        get_active() intentionally hides expired sessions from normal users.
        """
        with self._l:
            return self._s.get(gid)

    def get_active(self, gid):
        with self._l:
            s = self._s.get(gid)
            return s if s and not s.is_expired() else None

    def start(self, gid, gn, to, state=None):
        with self._l:
            n = time.time()
            sid = gn + "_" + gid + "_" + str(int(n))
            s = GameSession(gn, sid, gid, n, n + to, state or {})
            self._s[gid] = s
            return s

    def end(self, gid):
        with self._l:
            self._s.pop(gid, None)

    def expired_sessions(self):
        with self._l:
            return [(gid, s) for gid, s in self._s.items() if s.is_expired()]

    def cleanup(self):
        with self._l:
            for gid, s in list(self._s.items()):
                if s.is_expired():
                    del self._s[gid]


class EscrowEngine:
    def __init__(self):
        self._e = {}
        self._l = threading.Lock()

    def create(self, eid, total, owner, pool="game", exp=600):
        with self._l:
            if eid in self._e:
                return False
            self._e[eid] = {
                "id": eid,
                "total": total,
                "frozen": total,
                "owner": owner,
                "pool": pool,
                "status": "active",
                "entries": [],
                "created": time.time(),
                "expires": time.time() + exp,
            }
            return True

    def add_entry(self, eid, uid, amount):
        with self._l:
            e = self._e.get(eid)
            if not e or e["status"] != "active":
                return False
            e["total"] += amount
            e["frozen"] += amount
            e["entries"].append({"uid": uid, "amt": amount})
            return True

    def settle(self, eid, winners, fee=0.10):
        with self._l:
            e = self._e.get(eid)
            if not e:
                return None
            t = e["total"]
            sf = int(t * fee)
            pp = t - sf
            ps = [{"uid": uid, "amt": int(pp * sh / 100)} for uid, sh in winners]
            e["status"] = "settled"
            e["released"] = t
            return {"eid": eid, "total": t, "fee": sf, "prize": pp, "payouts": ps}

    def refund(self, eid):
        with self._l:
            e = self._e.get(eid)
            if not e:
                return None
            e["status"] = "refunded"
            return [(x["uid"], x["amt"]) for x in e["entries"]]


class GamePlugin(ABC):
    name = "base"
    display_name = "Base"
    description = ""
    tier = "daily"
    triggers = []
    group_required = False
    default_config = {
        "enabled": True,
        "reward_min": 5000,
        "reward_max": 25000,
        "cooldown_seconds": 0,
        "max_per_user_day": 1,
        "timeout_seconds": 180,
        "cost_quota": 0,
        "budget_pool": "game",
    }

    def __init__(self):
        self.config = dict(self.default_config)
        self._cnt = {}
        self._last = {}
        self._cnt_date = ""

    def on_load(self, c=None):
        if c:
            self.config.update(c)

    def match_command(self, text):
        tl = text.strip().lower()
        for t in self.triggers:
            if tl.startswith(t.lower()):
                return True
        return False

    def _maybe_reset_day(self):
        today = time.strftime("%Y-%m-%d")
        if self._cnt_date != today:
            self._cnt.clear()
            self._cnt_date = today

    def can_play(self, ctx, budget):
        if self.group_required and not getattr(ctx, "group_id", ""):
            return False, "group_only"
        if not self.config.get("enabled", True):
            return False, "disabled"
        self._maybe_reset_day()
        if self._cnt.get(ctx.user_id, 0) >= self.config.get("max_per_user_day", 1):
            return False, "今日次数已达上限"
        cd = self.config.get("cooldown_seconds", 0)
        if cd > 0 and time.time() - self._last.get(ctx.user_id, 0) < cd:
            return False, "cooldown"
        return True, ""

    def record_play(self, uid):
        self._cnt[uid] = self._cnt.get(uid, 0) + 1
        self._last[uid] = time.time()

    def get_admin_report(self, platform=None):
        """Per-plugin lightweight admin report."""
        return {
            "name": self.name,
            "display_name": self.display_name,
            "tier": self.tier,
            "enabled": bool(self.config.get("enabled", True)),
            "plays_today": int(sum(self._cnt.values())),
            "active_users": len(self._cnt),
            "platform": platform or "default",
        }

    def reset_daily(self):
        self._cnt.clear()
        self._last.clear()
        self._cnt_date = ""

    @abstractmethod
    def handle(self, ctx, sm, budget, escrow): ...


class GameDirector:
    # These commands are allowed before New API identity is resolved.
    # All quota-affecting / stateful games must pass through a bound New API user.
    IDENTITY_OPTIONAL_GAMES = {"verify", "invite", "admin_panel", "leaderboard"}
    INFO_ONLY_EXACT_COMMANDS = {
        "菜单",
        "游戏",
        "玩法",
        "帮助",
        "帮助游戏",
        "help",
        "menu",
        "game",
        "games",
        "游戏列表",
        "游戏菜单",
        "夺宝",
        "treasure",
        "彩票",
        "lottery",
        "状态",
        "排行榜",
        "排名",
        "leaderboard",
        "rank",
        "龙虎榜",
    }

    def __init__(self):
        self.sm = SessionManager()
        self.escrow = EscrowEngine()
        self.plugins = {}
        self._budget = None

    def init_budget(self, url, token):
        self._budget = BudgetChecker(url, token)

    def load_plugins(self, d=None):
        from game_plugins import discover_plugins

        self.plugins = discover_plugins(d)

    def _identity_required_reply(self, ctx):
        site = os.environ.get("PUBLIC_BASE_URL") or (
            f"https://ai.{ctx.site_id}.us.ci" if ctx.site_id else "站点"
        )
        bind_page = f"{site}/api/agent/chatops/bind-page"
        return GameResponse.quick(
            f"@{ctx.username} 需要先绑定账号才能参与游戏/领额度\n\n"
            f"\U0001f4cb 快速绑定（1分钟）：\n"
            f"1\ufe0f\u20e3 打开 {site} 注册或登录\n"
            f"2\ufe0f\u20e3 获取绑定码 \U0001f449 {bind_page}\n"
            f"3\ufe0f\u20e3 回群发送「绑定 你的绑定码」\n\n"
            f"\U0001f4a1 绑定后发「验牌」确认，即可签到/游戏"
        )

    def _is_info_only_command(self, ctx, game_name):
        text = str(getattr(ctx, "text", "") or "").strip().lower()
        raw = str(getattr(ctx, "text", "") or "").strip()
        if game_name in self.IDENTITY_OPTIONAL_GAMES:
            return True
        if raw in self.INFO_ONLY_EXACT_COMMANDS or text in {
            x.lower() for x in self.INFO_ONLY_EXACT_COMMANDS
        }:
            return True
        if game_name == "quiz" and text in (
            "答题",
            "每日答题",
            "答题帮助",
            "quiz",
            "helpquiz",
        ):
            return True
        if game_name == "fortune" and text in ("运势", "今日运势", "抽签", "fortune"):
            return True
        return False

    def _needs_identity(self, game_name, ctx=None):
        if ctx is not None and self._is_info_only_command(ctx, game_name):
            return False
        return game_name not in self.IDENTITY_OPTIONAL_GAMES

    def _has_identity(self, ctx):
        return bool(ctx.is_bound and int(ctx.new_api_user_id or 0) > 0)

    def _channel_of(self, ctx):
        if getattr(ctx, "platform", "") == "community":
            return "community"
        if getattr(ctx, "group_id", ""):
            return "group"
        return "private"

    def _channel_allowed(self, p, ctx):
        scope = p.config.get("channel_scope", "all")
        if not scope or scope == "all":
            return True
        return self._channel_of(ctx) in [s.strip() for s in str(scope).split(",")]

    def followup_plugin(self, ctx):
        for n, p in self.plugins.items():
            matcher = getattr(p, "match_followup", None)
            if callable(matcher):
                try:
                    if matcher(ctx):
                        return n, p
                except Exception:
                    continue
        return None, None

    def has_followup(self, ctx):
        n, _ = self.followup_plugin(ctx)
        return bool(n)

    def route(self, ctx):
        if self._budget:
            self._budget.set_platform(getattr(ctx, "platform", "") or "default")
        a = self.sm.get_active(ctx.group_id)
        matched_game = None
        matched_plugin = None
        for _n, _p in self.plugins.items():
            if _p.match_command(ctx.text):
                matched_game = _n
                matched_plugin = _p
                break
        followup_name, followup_plugin = self.followup_plugin(ctx)
        # Do not let an active round hijack explicit commands for another game.
        # Example: during 夺宝, users must still be able to send 邀请链接 / 福袋 / 我的 / 签到.
        # Active game still receives its own commands and free-form continuations such as 答A / 猜42.
        if a and a.game_name in self.plugins:
            active_name = a.game_name
            active_plugin = self.plugins.get(active_name)
            if not (matched_game and matched_game != active_name):
                if self._needs_identity(active_name, ctx) and not self._has_identity(
                    ctx
                ):
                    return self._identity_required_reply(ctx)
                resp = active_plugin.handle(ctx, self.sm, self._budget, self.escrow)
                if resp:
                    return resp
        if (
            followup_name
            and followup_plugin
            and not (matched_game and matched_game != followup_name)
        ):
            if not self._channel_allowed(followup_plugin, ctx):
                return GameResponse.quick(
                    f"@{ctx.username} 此玩法仅在指定渠道开放，当前渠道不可参与。"
                )
            if self._needs_identity(followup_name, ctx) and not self._has_identity(ctx):
                return self._identity_required_reply(ctx)
            return followup_plugin.handle(ctx, self.sm, self._budget, self.escrow)
        for n, p in self.plugins.items():
            if p.match_command(ctx.text):
                if not self._channel_allowed(p, ctx):
                    return GameResponse.quick(
                        f"@{ctx.username} 此玩法仅在指定渠道开放，当前渠道不可参与。"
                    )
                if self._needs_identity(n, ctx) and not self._has_identity(ctx):
                    return self._identity_required_reply(ctx)
                ok, why = p.can_play(ctx, self._budget)
                if not ok:
                    if why == "group_only":
                        return GameResponse.quick(
                            f"@{ctx.username} 此游戏仅限群聊中使用，请在QQ群或TG群中发送指令参与！"
                        )
                    if why == "disabled":
                        return GameResponse.quick(
                            f"@{ctx.username} 这个游戏当前未在本群/平台开放。"
                        )
                    return GameResponse.quick(f"@{ctx.username} {why}")
                return p.handle(ctx, self.sm, self._budget, self.escrow)
        tl = ctx.text.strip().lower()
        if tl.startswith(("rps ", "guess fist ", "duel_rps ", "猜拳")):
            return GameResponse.quick(
                f"@{ctx.username} 猜拳对决未开放或格式不对，试试: `猜拳 @对手 0.10`"
            )
        if tl.startswith(("banker ", "guess ", "坐庄", "开庄")):
            return GameResponse.quick(
                f"@{ctx.username} 坐庄猜数未开放，试试: `坐庄 0.10`"
            )
        return None

    def cleanup(self):
        self.sm.cleanup()
