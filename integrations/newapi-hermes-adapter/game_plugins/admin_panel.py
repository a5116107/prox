"""Admin Panel - handles admin DM commands for game/budget management."""

import json
from game_plugins.base import GameContext, GameResponse


class AdminPanel:
    """Handles admin private chat commands for game operations."""

    @staticmethod
    def handle(ctx, game_director, adapter_key=None):
        """Route admin commands. Returns GameResponse or None."""
        t = ctx.text.strip().lower()

        if t in ["game", "games", "games list", "game list", "/game"]:
            return AdminPanel._list_games(ctx, game_director)

        if t.startswith("game on ") or t.startswith("game enable "):
            game_name = t.replace("game on ", "").replace("game enable ", "").strip()
            return AdminPanel._toggle_game(ctx, game_director, game_name, True)

        if t.startswith("game off ") or t.startswith("game disable "):
            game_name = t.replace("game off ", "").replace("game disable ", "").strip()
            return AdminPanel._toggle_game(ctx, game_director, game_name, False)

        if t in ["budget", "pool", "budget report", "/budget"]:
            return AdminPanel._budget_report(ctx, game_director)

        if t in ["fund", "基金", "ops fund", "fund report", "/fund"]:
            return AdminPanel._fund_report(ctx)

        if t in ["sessions", "active", "active sessions", "/sessions"]:
            return AdminPanel._active_sessions(ctx, game_director)

        if t in ["help", "/help", "admin help", "admin commands"]:
            return AdminPanel._help(ctx)

        if t.startswith("grant ") or t.startswith("reward "):
            return AdminPanel._grant(ctx)

        return None

    @staticmethod
    def _list_games(ctx, director):
        games = director.plugins
        is_admin = bool(getattr(ctx, "is_admin", False))
        lines = (
            ["[Admin] Game List:", ""]
            if is_admin
            else ["🎮 当前可玩的活动/小游戏：", ""]
        )
        shown = 0
        for name, plugin in sorted(games.items()):
            enabled = plugin.config.get("enabled", False)
            if not is_admin and not enabled:
                continue
            shown += 1
            status = "ON" if enabled else "OFF"
            tier = plugin.tier
            desc = plugin.description
            if is_admin:
                lines.append(f"  {status} [{tier}] {plugin.display_name} ({name})")
            else:
                lines.append(f"- {plugin.display_name}")
            if desc:
                lines.append(f"  {desc}")
        if shown == 0 and not is_admin:
            lines.append("当前暂无开放小游戏。")
        if is_admin:
            lines.append(f"\nTotal: {len(games)} games loaded")
            lines.append("\nCommands: game on/off <name> | budget | fund | sessions")
        else:
            lines.append(
                "\n常用指令：签到 / 验牌 / treasure join / rps @对手 金额 / banker 金额"
            )
            lines.append("额度奖励以站点规则和管理员公告为准。")
        return GameResponse.quick("\n".join(lines))

    @staticmethod
    def _toggle_game(ctx, director, game_name, enable):
        plugin = director.plugins.get(game_name)
        if not plugin:
            return GameResponse.quick(
                f"[Admin] Game '{game_name}' not found.\n"
                f"Available: {', '.join(sorted(director.plugins.keys()))}"
            )

        plugin.config["enabled"] = enable
        status = "enabled" if enable else "disabled"

        try:
            from game_config_manager import get_config_manager

            cm = get_config_manager()
            platform = cm.get_platform(ctx) if hasattr(ctx, "platform") else "qq_group"
            site_id = getattr(ctx, "site_id", "") or ""
            cm.set_game_enabled(site_id, platform, game_name, enable)
        except Exception as e:
            print(f"[Admin] config persist failed: {e}", flush=True)

        return GameResponse.quick(
            f"[Admin] Game '{plugin.display_name}' ({game_name}) is now {status}.\n"
            f"Tier: {plugin.tier}\nConfig updated and persisted."
        )

    @staticmethod
    def _budget_report(ctx, director):
        bg = director._budget
        if not bg:
            return GameResponse.quick("【预算 / Budget】预算跟踪器尚未初始化。")

        scope = f"{getattr(ctx, 'site_id', '')}/{getattr(ctx, 'platform', '')}".strip(
            "/"
        )
        scope = scope or getattr(ctx, "platform", "") or "default"
        report = bg.get_admin_report(scope)
        summary = report.get("_summary", {})

        lines = [f"【预算观察 / Budget】{scope}", ""]
        for pool, item in report.items():
            if str(pool).startswith("_"):
                continue
            lines.append(
                f"- {pool}: 已用 ${item.get('spent_usd', 0):.2f} / 上限 ${item.get('limit_usd', 0):.2f} / "
                f"剩余 ${item.get('remaining_usd', 0):.2f} / 统计收入 ${item.get('income_usd', 0):.2f}"
            )
        lines.append("")
        lines.append(f"活跃用户: {summary.get('active_users', 0)}")
        lines.append(f"统计净额: ${summary.get('net_usd', 0):.2f}")
        lines.append("说明：真实发放由运营基金承担，收入字段只用于运营观察。")
        return GameResponse(
            reply="\n".join(lines),
            actions=[
                {
                    "type": "budget.check",
                    "target_type": "budget",
                    "reason": "admin_budget_report",
                }
            ],
            event="admin_budget_report",
        )

    @staticmethod
    def _fund_report(ctx):
        scope = f"{getattr(ctx, 'site_id', '')}/{getattr(ctx, 'platform', '')}".strip(
            "/"
        )
        scope = scope or getattr(ctx, "site_id", "") or "site"
        return GameResponse(
            reply=f"【运营基金 / Ops Fund】正在读取 {scope} 的实时基金余额、今日收支、资金来源、游戏/邀请/奖池/签到/群组审计…",
            actions=[
                {
                    "type": "fund.report.read",
                    "target_type": "ops_fund",
                    "reason": "admin_fund_report",
                }
            ],
            event="admin_fund_report",
        )

    @staticmethod
    def _active_sessions(ctx, director):
        sm = director.sm
        sessions = []
        with sm._l:
            for gid, sess in list(sm._s.items()):
                if not sess.is_expired():
                    left = max(0, int(sess.expires_at - __import__("time").time()))
                    sessions.append(
                        {
                            "gid": gid,
                            "game": sess.game_name,
                            "left": left,
                            "participants": len(
                                sess.state.get(
                                    "participants", sess.state.get("bets", {})
                                )
                            ),
                        }
                    )

        if not sessions:
            return GameResponse.quick("[Admin] No active game sessions.")

        lines = ["[Admin] Active Sessions:", ""]
        for s in sessions:
            lines.append(
                f"  [{s['game']}] Group: {s['gid']} | {s['participants']} players | {s['left']}s left"
            )
        lines.append(f"\n{sessions.__len__()} active session(s)")
        return GameResponse.quick("\n".join(lines))

    @staticmethod
    def _help(ctx):
        return GameResponse.quick(
            "【管理员命令 / Admin Commands】\n\n"
            "/game - 查看全部游戏\n"
            "game on <name> - 启用游戏\n"
            "game off <name> - 停用游戏\n"
            "/budget - 查看日控预算与统计收入\n"
            "/fund - 查看运营基金余额、闭环与可观测性\n"
            "/sessions - 查看进行中的游戏局\n"
            "grant <user_id> <amount_usd> [reason] - 给用户发放额度\n"
            "/help - 查看帮助\n\n"
            "以上命令仅支持管理员私聊使用。"
        )

    @staticmethod
    def _grant(ctx):
        # Parse: grant <user_id> <amount_usd> <reason>
        parts = ctx.text.strip().split()
        if len(parts) < 3:
            return GameResponse.quick(
                "【管理员发放 / Grant】格式：grant <user_id> <amount_usd> [reason]\n"
                "示例：grant 123 1.50 bonus_reward"
            )

        try:
            uid = int(parts[1])
            amount_usd = float(parts[2])
            reason = " ".join(parts[3:]) or "admin_grant"
        except:
            return GameResponse.quick(
                "【管理员发放 / Grant】参数错误，请使用：grant <user_id> <amount_usd>"
            )

        # Return action for New API to process
        amount_quota = int(amount_usd * 500000)
        return GameResponse(
            reply=f"【管理员发放 / Grant】准备给用户 #{uid} 发放 ${amount_usd:.2f}（{amount_quota} quota）\n原因：{reason}",
            actions=[
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": uid,
                    "quota_amount": amount_quota,
                    "budget_pool": "ops_comp",
                    "reason": reason,
                }
            ],
            event="admin_grant",
        )
