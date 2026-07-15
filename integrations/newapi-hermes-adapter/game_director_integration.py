"""Game Director Integration for Hermes Adapter."""

import os, sys, time, threading
from collections import defaultdict

PD = os.path.join(os.path.dirname(os.path.abspath(__file__)), "game_plugins")
if PD not in sys.path:
    sys.path.insert(0, os.path.dirname(PD))
from game_plugins.base import GameDirector, GameContext, GameResponse
from game_plugins.admin_panel import AdminPanel
from game_config_manager import get_config_manager
from activity_scheduler import ActivityScheduler

try:
    from db_logger import log_activity_plan, log_task_run
except ImportError:
    log_activity_plan = log_task_run = None

MODE_GAME = "game"
MODE_SERVICE = "service"
MODE_OPS = "ops"
MODE_ADMIN = "admin"


class GameDirectorIntegration:
    def __init__(self, site_id="default", site_name="New API", plugin_dir=None):
        self.site_id = site_id
        self.site_name = site_name
        self.pd = plugin_dir or PD
        self.director = GameDirector()
        self.director.load_plugins(self.pd)
        nb = os.environ.get("OPENAI_BASE_URL", "http://127.0.0.1:3000/v1").rstrip("/v1")
        nt = os.environ.get("OPENAI_API_KEY", "")
        self.director.init_budget(nb, nt)
        self._cfg()
        self._la = {}
        self._announce_queue = defaultdict(list)
        self._lock = threading.Lock()
        self._running = True
        self._cm = get_config_manager()
        self._active_groups = set()
        self._group_cooldown = {}
        self._send_fn = None  # set by adapter after init
        self._action_fn = None  # set by adapter after init
        self._leaderboard = self.director.plugins.get("leaderboard")
        self._scheduler = ActivityScheduler(
            send_fn=self._send_proactive, site_id=site_id
        )
        self._bg = threading.Thread(target=self._bg_loop, daemon=True)
        self._bg.start()
        self._sys_cfg = self._load_sys_cfg()
        print(
            f"[Director] site={site_id} games={len(self.director.plugins)} bg_announcer=on",
            flush=True,
        )

    def _load_sys_cfg(self):
        try:
            import json as _j

            with open("/opt/newapi-hermes-adapter/game_config.json") as _f:
                return _j.load(_f).get("system", {})
        except Exception:
            return {}

    def _cfg(self):
        from game_config_manager import get_config_manager

        self._cm = get_config_manager()
        # Load all plugin configs from config file (not env vars)
        for n, p in self.director.plugins.items():
            if n == "verify":
                p.config["site_url"] = os.environ.get(
                    "PUBLIC_BASE_URL", f"https://ai.{self.site_id}.us.ci"
                )
                p.config["api_url"] = (
                    os.environ.get("PUBLIC_API_BASE_URL", "")
                    or os.environ.get("API_BASE_URL", "")
                    or p.config["site_url"].replace("https://ai.", "https://api.")
                )
                p.config["community_host"] = os.environ.get(
                    "COMMUNITY_HOST", "https://dc.hhhl.cc"
                )

    def classify_text(self, text):
        """Quick check: does text match any game plugin trigger? Returns game name or None."""
        if not self.director:
            return None
        t = str(text or "").strip().lower()
        if not t:
            return None
        # Also match game menu keywords
        if self._is_menu_command(t):
            return "menu"
        for n, p in self.director.plugins.items():
            if p.match_command(t):
                return n
        return None

    def classify(self, ctx):
        t = ctx.text.strip().lower()
        if t.startswith(
            (
                "rps ",
                "guess fist ",
                "duel_rps ",
                "banker ",
                "guess ",
                "猜拳",
                "比大小",
                "坐庄",
                "开庄",
                "猜 ",
                "开奖",
                "接受",
                "应战",
            )
        ):
            for n, p in self.director.plugins.items():
                if p.match_command(t):
                    if self._cm and not self._cm.is_game_enabled(ctx, n):
                        continue
                    return MODE_GAME
        for n, p in self.director.plugins.items():
            if p.match_command(t):
                if self._cm and not self._cm.is_game_enabled(ctx, n):
                    continue
                return MODE_GAME
        if self._is_menu_command(t):
            return MODE_ADMIN
        if ctx.is_admin and not ctx.group_id:
            admin_cmds = [
                "budget",
                "pool",
                "/budget",
                "sessions",
                "active",
                "/sessions",
                "help",
                "/help",
                "grant",
                "reward",
                "ban",
                "risk",
                "user",
                "status",
                "log",
                "restart",
                "health",
                "docker",
                "service",
                "game on",
                "game off",
                "game enable",
                "game disable",
            ]
            for cmd in admin_cmds:
                if t.startswith(cmd):
                    return MODE_ADMIN
        if t.startswith(
            (
                "rps ",
                "guess fist ",
                "duel_rps ",
                "banker ",
                "guess ",
                "猜拳",
                "比大小",
                "坐庄",
                "开庄",
                "猜 ",
                "开奖",
                "接受",
                "应战",
            )
        ):
            for n, p in self.director.plugins.items():
                if p.match_command(t):
                    if self._cm and not self._cm.is_game_enabled(ctx, n):
                        continue
                    return MODE_GAME
        if self.director and self.director.has_followup(ctx):
            return MODE_GAME
        return MODE_SERVICE

    def is_game_followup(self, text, platform="", group_id="", user_id=""):
        if not self.director:
            return False
        ctx = GameContext(
            site_id=self.site_id,
            site_name=self.site_name,
            platform=platform or "unknown",
            group_id=str(group_id or ""),
            user_id=str(user_id or ""),
            username=str(user_id or ""),
            text=str(text or ""),
        )
        try:
            return bool(self.director.has_followup(ctx))
        except Exception:
            return False

    def _is_menu_command(self, text):
        t = str(text or "").strip().lower()
        return t in (
            "game",
            "games",
            "games list",
            "game list",
            "/game",
            "/games",
            "/help",
            "help",
            "menu",
            "/menu",
            "游戏",
            "游戏列表",
            "游戏菜单",
            "玩法",
            "帮助",
            "菜单",
            "帮助游戏",
            "/游戏",
        )

    def _game_help_hint(self, game_name):
        hints = {
            "menu": (
                "游戏",
                "返回完整玩法菜单、触发词、示例和注意事项",
                "先看菜单再按示例操作",
            ),
            "verify": (
                "验牌",
                "返回账号绑定/API Key/额度三项结果；失败会给原因和下一步",
                "参与下注/对战前必须先验牌",
            ),
            "checkin": (
                "签到",
                "返回本次奖励、连续天数和当前余额；每日一次",
                "站点签到已迁移到群内签到",
            ),
            "invite": (
                "邀请链接",
                "返回专属邀请链接/统计；新用户验牌后自动结算奖励",
                "邀请关系以 New API 用户为准",
            ),
            "fortune": (
                "运势",
                "返回今日签文、运气值和提示；通常每日一次",
                "免费轻互动",
            ),
            "quiz": (
                "答题；答 A；答:太平洋",
                "先出题，再用「答 A/B/C/D」或「答:答案」作答，答对发奖励",
                "答案格式很重要",
            ),
            "treasure": (
                "夺宝；加入夺宝 0.10",
                "返回奖池/人数；开奖后展示名次和奖励，不足人数自动退款",
                "只发「夺宝」看规则，不扣款；加入才扣款",
            ),
            "dice": (
                "骰子 大 0.10；骰子 豹子 0.05",
                "返回点数、输赢、扣款/奖励和余额",
                "先写玩法再写金额",
            ),
            "lottery": (
                "机选；彩票 1 2 3 4 5 6 + 7",
                "返回彩票号码；开奖后返回中奖等级和奖金",
                "手选格式：6个红球 + 1个蓝球",
            ),
            "redpacket": (
                "红包雨 5 1.00；抢红包",
                "返回红包总额/剩余数量；抢到后返回领取金额",
                "通常由管理员发起",
            ),
            "duel_rps": (
                "猜拳 @对手 0.10；接受 石头",
                "返回挑战、双方出拳、胜负和结算",
                "PVP 群聊玩法",
            ),
            "duel_compare": (
                "比大小 @对手 0.10；接受",
                "双方抽数比大小，返回点数、胜负和结算",
                "PVP 群聊玩法",
            ),
            "duel_idiom": (
                "成语接龙 @对手；接龙 画龙点睛",
                "返回轮到谁、当前成语，结束时给胜负/平局",
                "PVP 群聊玩法",
            ),
            "banker_guess": (
                "坐庄 0.50；猜 42；开奖",
                "返回庄家奖池、猜测记录，开奖时给最接近者和结算",
                "系统坐庄/庄家玩法",
            ),
            "bounty": (
                "发布悬赏 0.50 修复一个问题；接单 1；完成悬赏 1",
                "返回任务编号、接单人、完成/退款结果",
                "任务型玩法",
            ),
            "predict": (
                "竞猜 今天会涨吗；押 A 0.10；竞猜结算 A",
                "返回盘口、下注记录、结算结果",
                "一般由管理员开盘/结算",
            ),
            "wheel": (
                "转盘",
                "返回抽中档位、奖励/扣款和当前余额",
                "每日免费次数用完后可能扣费",
            ),
            "leaderboard": ("排行榜", "返回本群游戏排行榜", "只查数据不扣款"),
            "profile": ("我的", "返回余额、统计和绑定状态", "只查数据不扣款"),
            "luckybag": ("福袋", "返回随机奖励/结果", "每日福利型玩法"),
        }
        return hints.get(
            game_name,
            ("发送触发词或查看菜单", "机器人返回规则、处理结果或失败原因", ""),
        )

    def _game_menu_reply(self, ctx):
        lines = [f"🎮 {self.site_name} 群聊游戏菜单"]
        lines.append(
            "发送「触发词 + 参数」即可参与；涉及额度/奖励的玩法会先要求「验牌」。"
        )
        lines.append("菜单是系统入口，不算游戏开关；下面只展示本群当前已开放玩法。")
        lines.append(
            "不知道格式时直接照下面示例发，机器人会返回通过/失败原因、扣款、奖励和余额。"
        )
        enabled = []
        disabled = []
        enabled_names = []
        for n, p in sorted(self.director.plugins.items()):
            try:
                is_on = (
                    self._cm.is_game_enabled(ctx, n)
                    if self._cm
                    else p.config.get("enabled", True)
                )
            except Exception:
                is_on = p.config.get("enabled", True)
            triggers = ", ".join((getattr(p, "triggers", []) or [])[:4])
            display = getattr(p, "display_name", n)
            item = f"{'✅' if is_on else '⛔'} {display} / {n}" + (
                f"：{triggers}" if triggers else ""
            )
            (enabled if is_on else disabled).append(item)
            if is_on:
                enabled_names.append((n, display))
        if enabled:
            lines.append("\n已开放：")
            lines.extend(enabled[:30])
        # 普通用户菜单只展示已开放玩法；未开放/待配置项只允许管理员调试时看到，避免误导用户。
        if disabled and bool(getattr(ctx, "is_admin", False)):
            lines.append("\n未开放/待配置：")
            lines.extend(disabled[:30])
        lines.append("\n常用格式示例：")
        preferred = [
            "verify",
            "checkin",
            "invite",
            "fortune",
            "quiz",
            "treasure",
            "lottery",
            "dice",
            "wheel",
            "leaderboard",
            "profile",
            "luckybag",
            "redpacket",
            "duel_rps",
            "duel_compare",
            "banker_guess",
            "bounty",
            "predict",
            "duel_idiom",
        ]
        enabled_map = {n: d for n, d in enabled_names}
        shown = []
        for n in preferred + [n for n, _ in enabled_names]:
            if n in enabled_map and n not in shown:
                example, result, note = self._game_help_hint(n)
                suffix = f"；{note}" if note else ""
                lines.append(f"  - {enabled_map[n]}：发「{example}」→ {result}{suffix}")
                shown.append(n)
            if len(shown) >= 18:
                break
        lines.append("\n结果怎么看：")
        lines.append("  ✅ 成功：会写清本次动作、获得/扣除额度、当前余额或奖池。")
        lines.append(
            "  ❌ 失败：会写清原因，例如未绑定、未验牌、余额不足、格式错误、当日次数已用完。"
        )
        lines.append(
            "  🧩 多步玩法：先发起，再按返回里的下一步继续，例如「答 A」「猜 42」「接受」。"
        )
        lines.append("\n常用入口：验牌、签到、邀请链接、运势、答题、夺宝、排行榜。")
        return "\n".join(lines)

    def handle(self, raw):
        gid = str(raw.get("group_id", ""))
        ctx = GameContext(
            site_id=raw.get("site_id", self.site_id),
            site_name=raw.get("site_name", self.site_name),
            platform=raw.get("platform", "unknown"),
            group_id=gid,
            user_id=str(raw.get("user_id", "")),
            username=str(raw.get("username", "")),
            new_api_user_id=raw.get("new_api_user_id", 0) or 0,
            new_api_token=raw.get("new_api_token", "") or "",
            text=str(raw.get("text", "")),
            quota_balance=raw.get("quota_balance", 0) or 0,
            is_admin=raw.get("is_admin", False),
            is_bound=raw.get("user_bound", False),
        )
        if self._cm and self._cm._reload_if_changed():
            self._sys_cfg = self._load_sys_cfg()
        if not self._sys_cfg.get("games_enabled", True):
            return None
        mode = self.classify(ctx)
        if ctx.group_id:
            # QQ/TG 的主动推送目标是数字群号/聊天 ID；社区 room_id 是 ani... 字符串，
            # 社区回复由 New API 通过社区 Bot Token 发送，不能误走 OneBot/TG 主动推送。
            if str(ctx.group_id).strip().lstrip("-").isdigit():
                self._active_groups.add(ctx.group_id)
                self._scheduler.register_group(ctx.group_id)
            now = time.time()
            _ck = (ctx.group_id, ctx.user_id)
            last = self._group_cooldown.get(_ck, 0)
            if now - last < 3 and mode == MODE_GAME:
                # Game commands often require immediate follow-up messages
                # (e.g. 验牌 -> 签到, 红包雨 -> 红包).  Suppressing them here
                # makes the adapter fall through to LLM chat and can create
                # fake "success" replies without quota mutations.  Let each
                # game plugin enforce its own duplicate/daily limits instead.
                pass
            else:
                self._group_cooldown[_ck] = now
        if self._is_menu_command(ctx.text):
            # 菜单是系统入口，不参与 game_config 的启停；但菜单内容只展示当前群组已开放玩法。
            return self._fmt(
                GameResponse.quick(self._game_menu_reply(ctx), event="game_menu"),
                MODE_ADMIN,
                "game_menu",
            )
        if self.director and self.director.has_followup(ctx):
            mode = MODE_GAME
        if mode == MODE_GAME:
            # Apply per-platform config before routing
            if self._cm:
                t = ctx.text.strip().lower()
                for n, p in self.director.plugins.items():
                    if p.match_command(t):
                        self._cm.apply_game_config(ctx, p)
                        break
            resp = self.director.route(ctx)
            if resp:
                if (
                    self._leaderboard
                    and resp.event
                    and resp.event not in ("game_menu", "error", "group_only")
                ):
                    won = (
                        "win" in (resp.event or "").lower()
                        or "赢" in (resp.reply or "")
                        or "获胜" in (resp.reply or "")
                    )
                    self._leaderboard.record(
                        ctx.user_id,
                        ctx.username,
                        resp.event.split("_")[0] if resp.event else "unknown",
                        won=won,
                    )
                # Invite reward on verify_pass
                if resp.event == "verify_pass" and ctx.user_id:
                    self._process_invite_reward(ctx, resp)
                return self._fmt(resp, mode, resp.event)
        if mode in (MODE_ADMIN, MODE_OPS):
            admin_resp = AdminPanel.handle(ctx, self.director)
            if admin_resp:
                return self._fmt(admin_resp, mode, admin_resp.event)
        return None

    def handle_notice(
        self, notice_type, gid, uid, sub_type=None, extra=None, operator_id=None
    ):
        """Handle platform notice events (group join/leave). Returns response or None."""
        if not self.director:
            return None
        # Forward to plugins that handle notices
        for n, p in self.director.plugins.items():
            if hasattr(p, "handle_notice_increase") and notice_type == "group_increase":
                try:
                    resp = p.handle_notice_increase(gid, uid, operator_id)
                    if resp:
                        return self._fmt(resp, "game", getattr(resp, "event", ""))
                except Exception:
                    pass
            if hasattr(p, "handle_notice_decrease") and notice_type in (
                "group_decrease",
                "leave",
                "kick",
            ):
                try:
                    resp = p.handle_notice_decrease(
                        gid,
                        uid,
                        sub_type=sub_type,
                        operator_id=operator_id,
                        extra=extra,
                    )
                    if resp:
                        return self._fmt(resp, "game", getattr(resp, "event", ""))
                except TypeError:
                    try:
                        resp = p.handle_notice_decrease(gid, uid, operator_id)
                        if resp:
                            return self._fmt(resp, "game", getattr(resp, "event", ""))
                    except Exception:
                        pass
                except Exception:
                    pass
        return None

    def _process_invite_reward(self, ctx, resp):
        """After verify_pass, ask New API invite endpoint to settle rewards atomically."""
        if not self._sys_cfg.get("invite_reward_enabled", True):
            return
        try:
            invite_p = self.director.plugins.get("invite")
            if not invite_p or not hasattr(invite_p, "check_verify_reward"):
                return
            result = invite_p.check_verify_reward(
                ctx.user_id,
                ctx.username,
                ctx.new_api_user_id,
                ctx.group_id,
                ctx.platform,
            )
            if not result:
                return
            if isinstance(result, dict):
                if not result.get("success") or not result.get("awarded"):
                    return
                budget = self.director._budget
                inviter_reward = int(result.get("inviter_reward_quota") or 0)
                invitee_reward = int(result.get("invitee_reward_quota") or 0)
                inviter_usd = budget.quota_to_usd(inviter_reward)
                invitee_usd = budget.quota_to_usd(invitee_reward)
                resp.reply += f"\n\n🎉 邀请奖励已发放！\n  邀请人 +${inviter_usd:.2f} / 你 +${invitee_usd:.2f}"
                if ctx.group_id and self._send_fn:
                    self._send_proactive(
                        ctx.group_id,
                        f"🎊 @{ctx.username} 已验牌成功，邀请奖励已由系统自动结算。",
                    )
                return
            print(
                f"[Director] ignore legacy invite reward result type={type(result).__name__}",
                flush=True,
            )
        except Exception as e:
            print(f"[Director] invite reward error: {e}", flush=True)

    def get_game_rules_prompt(self, platform="qq"):
        """Generate a rules/info block for the AI agent system prompt."""
        if not self.director or not self._cm:
            return ""
        import json as _j

        cfg_path = getattr(self._cm, "_config_path", "")
        try:
            full_cfg = _j.load(open(cfg_path)) if cfg_path else {}
        except Exception:
            full_cfg = {}
        site_cfg = full_cfg.get("sites", {}).get(self.site_id, {})
        plat_cfg = site_cfg.get("platforms", {}).get(platform, {})
        games_cfg = plat_cfg.get("games", {})

        budget = self.director._budget
        lines = []
        lines.append(f"[当前平台游戏与功能 - {self.site_name} / {platform}]")
        lines.append("")
        lines.append("【核心流程】")
        lines.append(
            "1. 绑定：新用户在站点注册后获得「绑定码」，在群里发送「绑定 <绑定码>」完成账号关联。"
        )
        lines.append(
            "2. 验牌：发送「验牌」检查 ①账号绑定 ②API Key ③额度状态；全部通过后才能参与扣额度/发奖励玩法。"
        )
        lines.append(
            "3. 签到：每日在对应群里发送「签到」领取额度奖励；站点内签到会引导到社区/QQ/TG 群签到。"
        )
        lines.append(
            "4. 邀请：邀请好友进群或使用注册链接；好友注册并验牌成功后，由 New API 权威接口结算奖励。"
        )
        lines.append(
            "5. 多步玩法：机器人返回下一步后，用户必须按固定格式继续，例如「答 A」「猜 42」「接受 石头」。"
        )

        inv_cfg = games_cfg.get("invite", {})
        inviter_q = inv_cfg.get("inviter_reward_quota", 1500000)
        invitee_q = inv_cfg.get("invitee_reward_quota", 750000)
        lines.append("")
        lines.append("【邀请奖励】")
        lines.append(
            f"  群组邀请：邀请人获 ${budget.quota_to_usd(inviter_q):.2f}，被邀人获 ${budget.quota_to_usd(invitee_q):.2f}"
        )
        lines.append(
            "  站点链接邀请：邀请人获 $30，被邀人获 $5（通过站点注册链接 ?aff=ID）"
        )
        lines.append(
            "  回复用户时必须说明：奖励以“新用户完成注册 + 验牌通过”为准，不是单纯进群就发。"
        )

        ci_cfg = games_cfg.get("checkin", {})
        ci_min = ci_cfg.get("reward_min", 250000)
        ci_max = ci_cfg.get("reward_max", 1000000)
        ci_bonus = ci_cfg.get("bonus_extra", 500000)
        lines.append("")
        lines.append("【签到奖励】")
        lines.append(
            f"  每日签到：${budget.quota_to_usd(ci_min):.2f} ~ ${budget.quota_to_usd(ci_max):.2f}"
        )
        lines.append(f"  7天连续额外奖励：${budget.quota_to_usd(ci_bonus):.2f}")
        lines.append("  标准成功结果应包含：获得额度、当前余额、连续天数/当日状态。")
        lines.append(
            "  标准失败结果应包含：已签到/未绑定/未验牌/功能关闭/预算不足等明确原因。"
        )

        lines.append("")
        lines.append("【可用游戏列表：规则 + 示例 + 返回结果】")
        enabled_games = []
        for gname, p in sorted(self.director.plugins.items()):
            gcfg = games_cfg.get(gname, {})
            is_on = gcfg.get("enabled", p.config.get("enabled", True))
            if not is_on:
                continue
            triggers = ", ".join((getattr(p, "triggers", []) or [])[:4])
            desc = getattr(p, "description", "")
            display = getattr(p, "display_name", gname)
            example, result, note = self._game_help_hint(gname)
            line = f"  {display}({gname}): {desc}"
            if triggers:
                line += f" | 触发词: {triggers}"
            line += f" | 示例: {example} | 返回: {result}"
            if note:
                line += f" | 注意: {note}"
            if gname == "dice":
                line += " | 经济参数: 下注$0.01-$0.50，大/小约1.9倍，豹子约30倍"
            elif gname == "treasure":
                cost = gcfg.get("entry_cost_quota", 500000)
                line += (
                    f" | 经济参数: 入场约${budget.quota_to_usd(cost):.2f}，最少3人开奖"
                )
            elif gname in ("duel_rps", "duel_compare"):
                line += " | 经济参数: 约15%平台佣金"
            elif gname == "quiz":
                qr = gcfg.get("reward_quota", 100000)
                line += f" | 经济参数: 答对奖约${budget.quota_to_usd(qr):.2f}，答案需「答 A」或「答:答案」"
            elif gname == "fortune":
                line += " | 经济参数: 每日一次免费"
            elif gname == "wheel":
                line += " | 经济参数: 每日免费次数后可能扣费"
            elif gname == "lottery":
                line += " | 经济参数: 约$0.05/注，约15%抽佣"
            elif gname == "bounty":
                line += " | 经济参数: 常用$0.10-$2.00，超时/取消按规则退款"
            elif gname == "predict":
                line += " | 经济参数: 竞猜A/B方，约15%佣金"
            enabled_games.append(line)
        lines.extend(enabled_games)

        lines.append("")
        lines.append("【用户输入示例】")
        lines.append("  基础：验牌｜签到｜邀请链接｜排行榜｜我的")
        lines.append("  答题：答题 → 答 A / 答:太平洋")
        lines.append("  夺宝：夺宝 → 加入夺宝 0.10")
        lines.append("  彩票：机选 / 彩票 1 2 3 4 5 6 + 7")
        lines.append("  对战：猜拳 @对手 0.10 → 接受 石头；比大小 @对手 0.10 → 接受")
        lines.append("  坐庄：坐庄 0.50 → 猜 42 → 开奖")
        lines.append("  红包：红包雨 5 1.00 → 抢红包")
        lines.append("  竞猜：竞猜 今天会涨吗 → 押 A 0.10 → 竞猜结算 A")
        lines.append("")
        lines.append("【回复/结果格式要求】")
        lines.append(
            "  1. 不要只说“已处理”；必须说明用户该发什么格式、系统做了什么、下一步是什么。"
        )
        lines.append(
            "  2. 涉及额度时必须展示：扣除/获得额度、奖池或当前余额；失败时展示具体原因。"
        )
        lines.append(
            "  3. 多步游戏必须明确下一步示例，例如“请发送：答 A”或“请发送：猜 42”。"
        )
        lines.append(
            "  4. 未开放/无权限/未验牌/未绑定时，给明确引导，不要编造不存在的功能。"
        )
        lines.append(
            "  5. PVP/庄家/竞猜类游戏只在群聊中使用；社区副主场如关闭重玩法，应引导去主群。"
        )
        return "\n".join(lines)

    def _fmt(self, resp, mode, event=""):
        return {
            "reply": resp.reply,
            "risk": "low",
            "requires_approval": False,
            "actions": resp.actions,
            "notes": f"mode:{mode}|evt:{event}|game_handled:true",
            "game_handled": True,
        }

    def set_send_fn(self, fn):
        self._send_fn = fn
        self._scheduler.send_fn = fn

    def set_action_fn(self, fn):
        self._action_fn = fn

    def _send_proactive(self, gid, text):
        gid_s = str(gid or "").strip()
        if gid_s and not gid_s.lstrip("-").isdigit():
            print(
                f"[Director] proactive send skipped non-platform thread={gid_s}",
                flush=True,
            )
            return
        if self._send_fn:
            self._send_fn(gid_s or gid, text)

    def _bg_loop(self):
        """Background thread: cleanup sessions + generate treasure announcements."""
        while self._running:
            time.sleep(15)
            try:
                sm = self.director.sm
                buds = self.director._budget
                expired = list(sm.expired_sessions())
                from game_plugins.treasure import TreasureGame

                for gid, s in expired:
                    p = self.director.plugins.get(getattr(s, "game_name", ""))
                    if not p:
                        continue
                    try:
                        platform = str(
                            (s.state or {}).get("platform")
                            or os.environ.get("BOT_PLATFORM")
                            or "unknown"
                        )
                        actor_uid = "system"
                        resp = None
                        if s.game_name == "redpacket":
                            actor_uid = str(
                                (s.state or {}).get("owner_user_id") or "system"
                            )
                            ctx = GameContext(
                                site_id=self.site_id,
                                platform=platform,
                                group_id=gid,
                                user_id=actor_uid,
                                username=str(
                                    (s.state or {}).get("owner_username") or actor_uid
                                ),
                                new_api_user_id=int(
                                    (s.state or {}).get("owner_new_api_user_id") or 0
                                ),
                                new_api_token="",
                                text="__auto_close__",
                                quota_balance=0,
                                is_admin=True,
                                is_bound=True,
                            )
                            resp = p.handle(ctx, sm, buds, self.director.escrow)
                        elif (
                            s.game_name == "treasure"
                            or getattr(p, "name", "") == "treasure"
                        ):
                            state = s.state or {}
                            participants = state.get("participants") or {}
                            if participants:
                                actor_uid = str(next(iter(participants.keys())))
                            print(
                                f"[Director] expired treasure gid={gid} platform={platform} participants={len(participants)} pool={int(state.get('pool') or 0)} actor={actor_uid}",
                                flush=True,
                            )
                            ctx = GameContext(
                                site_id=self.site_id,
                                platform=platform,
                                group_id=gid,
                                user_id=actor_uid,
                                username=str(
                                    (participants.get(actor_uid) or {}).get("username")
                                    or actor_uid
                                ),
                                new_api_user_id=int(
                                    (participants.get(actor_uid) or {}).get("napi") or 0
                                ),
                                new_api_token="",
                                text="__auto_draw__",
                                quota_balance=0,
                                is_admin=True,
                                is_bound=bool(participants),
                            )
                            if hasattr(p, "_finalize"):
                                resp = p._finalize(
                                    ctx, sm, state, buds, end_session=False
                                )
                            else:
                                resp = p.handle(ctx, sm, buds, self.director.escrow)
                        else:
                            resp = None
                        action_count = (
                            len(getattr(resp, "actions", []) or []) if resp else 0
                        )
                        event_name = getattr(resp, "event", "") if resp else ""
                        print(
                            f"[Director] expired finalized gid={gid} game={s.game_name} event={event_name} actions={action_count}",
                            flush=True,
                        )
                        if resp and resp.reply:
                            self._send_proactive(gid, resp.reply)
                        if resp and resp.actions and self._action_fn:
                            try:
                                dispatch_result = self._action_fn(
                                    resp.actions, platform, gid, actor_uid
                                )
                                print(
                                    f"[Director] expired dispatch gid={gid} game={s.game_name} event={event_name} result={str(dispatch_result)[:500]}",
                                    flush=True,
                                )
                            except Exception as e:
                                print(
                                    f"[Director] action dispatch error gid={gid}: {e}",
                                    flush=True,
                                )
                    except Exception as e:
                        print(
                            f"[Director] expired session handling error gid={gid}: {e}",
                            flush=True,
                        )
                    finally:
                        try:
                            sm.end(gid)
                        except Exception:
                            pass
                self.director.cleanup()
                buds = self.director._budget
                from game_plugins.treasure import TreasureGame

                for n, p in self.director.plugins.items():
                    if getattr(p, "name", "") != "treasure":
                        continue
                    for gid in list(sm._s.keys()):
                        s = sm._s.get(gid)
                        if s and not s.is_expired() and s.game_name == "treasure":
                            ann = p._check_announce(gid, sm, buds)
                            if ann:
                                with self._lock:
                                    self._announce_queue[gid].append(ann)
                            # Also check if round expired (should draw)
                            elapsed = time.time() - s.state.get(
                                "started_at", s.started_at
                            )
                            if elapsed >= p.config["round_duration_sec"]:
                                ctx = GameContext(
                                    site_id=self.site_id,
                                    platform=os.environ.get("BOT_PLATFORM")
                                    or "unknown",
                                    group_id=gid,
                                    user_id="system",
                                    username="system",
                                    text="__auto_draw__",
                                    is_admin=True,
                                    is_bound=False,
                                )
                                if hasattr(p, "_finalize"):
                                    resp = p._finalize(
                                        ctx, sm, s.state or {}, buds, end_session=False
                                    )
                                else:
                                    resp = p.handle(ctx, sm, buds, self.director.escrow)
                                if resp and resp.reply:
                                    self._send_proactive(gid, resp.reply)
                                if resp and resp.actions and self._action_fn:
                                    state_participants = (s.state or {}).get(
                                        "participants"
                                    ) or {}
                                    dispatch_actor = (
                                        str(next(iter(state_participants.keys())))
                                        if state_participants
                                        else "system"
                                    )
                                    dispatch_result = self._action_fn(
                                        resp.actions,
                                        os.environ.get("BOT_PLATFORM") or "unknown",
                                        gid,
                                        dispatch_actor,
                                    )
                                    print(
                                        f"[Director] treasure elapsed dispatch gid={gid} actor={dispatch_actor} actions={len(resp.actions or [])} result={str(dispatch_result)[:500]}",
                                        flush=True,
                                    )
                                try:
                                    sm.end(gid)
                                except Exception:
                                    pass
                # --- Treasure auto-start ---
                treasure_p = self.director.plugins.get("treasure")
                if treasure_p and treasure_p.config.get("auto_start_enabled", False):
                    interval = treasure_p.config.get("auto_start_interval_sec", 3600)
                    for gid in list(self._active_groups):
                        if not sm.get_active(gid):
                            last_end = getattr(treasure_p, "_last_round_end", {}).get(
                                gid, 0
                            )
                            if time.time() - last_end >= interval:
                                try:
                                    dur = treasure_p.config.get(
                                        "round_duration_sec", 300
                                    )
                                    cost = treasure_p.config.get(
                                        "entry_cost_quota", 2500000
                                    )
                                    state = {
                                        "type": "treasure",
                                        "participants": {},
                                        "pool": 0,
                                        "acc_rounds": getattr(
                                            treasure_p, "_ar", {}
                                        ).get(gid, 0)
                                        + 1,
                                        "started_at": time.time(),
                                        "platform": os.environ.get("BOT_PLATFORM")
                                        or "unknown",
                                    }
                                    sm.start(gid, "treasure", dur, state)
                                    ann = f"\U0001f3c6 夺宝奇兵自动开启！\n\U0001f4b0 入场费 ${buds.quota_to_usd(cost):.2f}\n\u23f0 {dur // 60}分钟后开奖\n\n发送「加入夺宝」参与！"
                                    self._send_proactive(gid, ann)
                                    if not hasattr(treasure_p, "_last_round_end"):
                                        treasure_p._last_round_end = {}
                                    treasure_p._last_round_end[gid] = time.time()
                                    if log_task_run:
                                        log_task_run(
                                            self.site_id,
                                            run_type="treasure_auto_start",
                                            status="completed",
                                            output_data={
                                                "group_id": gid,
                                                "duration_sec": dur,
                                            },
                                        )
                                except Exception as ae:
                                    print(
                                        f"[Director] auto-start error gid={gid}: {ae}",
                                        flush=True,
                                    )

                # --- Activity scheduler tick ---
                self._scheduler.tick()
                # Flush persistent state
                try:
                    from state_store import get_store

                    get_store().flush_if_dirty()
                except Exception:
                    pass
            except Exception as e:
                print(f"[Director] bg error: {e}", flush=True)

    def drain_announcements(self, gid):
        """Get pending announcements for a group. Call on any message."""
        with self._lock:
            if gid in self._announce_queue and self._announce_queue[gid]:
                anns = list(self._announce_queue[gid])
                self._announce_queue[gid].clear()
                return anns
        return []

    def list_games(self):
        from game_plugins import list_plugins

        return list_plugins()


_i = None


def get_director(site_id=None, site_name=None, plugin_dir=None):
    global _i
    if _i is None:
        _i = GameDirectorIntegration(
            site_id or os.environ.get("SITE_ID", "default"),
            site_name or os.environ.get("SITE_NAME", "New API"),
            plugin_dir,
        )
    return _i
