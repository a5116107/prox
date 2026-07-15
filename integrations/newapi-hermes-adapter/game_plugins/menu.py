"""Game Menu - categorized list of available games."""

import os
from .base import GamePlugin, GameContext, GameResponse, format_card

CATEGORIES = [
    (
        "\U0001f381",
        "福利日常",
        [
            ("checkin", "签到"),
            ("luckybag", "福袋"),
            ("fortune", "运势/抽签"),
            ("wheel", "转盘"),
        ],
    ),
    (
        "\U0001f3ae",
        "娱乐游戏",
        [
            ("dice", "骰子 <金额>"),
            ("lottery", "买彩票/开奖"),
            ("quiz", "答题"),
        ],
    ),
    (
        "⚔️",
        "对战PvP",
        [
            ("duel_rps", "猜拳 @对手 <金额>"),
            ("duel_compare", "比大小 @对手 <金额>"),
            ("treasure", "寻宝"),
            ("banker_guess", "坐庄 <金额>"),
            ("bounty", "悬赏 <金额> <问题>"),
            ("predict", "竞猜"),
        ],
    ),
    (
        "\U0001f9ed",
        "实用工具",
        [
            ("invite", "邀请/邀请链接"),
            ("verify", "验牌"),
            ("leaderboard", "排行榜"),
            ("profile", "我的/资产"),
        ],
    ),
]


class MenuGame(GamePlugin):
    name = "menu"
    display_name = "游戏菜单"
    description = "展示所有可用游戏和触发关键词"
    tier = "utility"
    triggers = ["菜单", "游戏", "help", "menu", "帮助"]
    default_config = {
        "enabled": True,
        "max_per_user_day": 99,
        "cooldown_seconds": 3,
        "budget_pool": "game",
    }

    def handle(self, ctx, sm, budget, escrow):
        from game_plugins import get_plugin

        site_name = os.environ.get("SITE_NAME", "小助手")
        rows = []
        for cat_emoji, cat_name, games in CATEGORIES:
            enabled = []
            for gname, trigger in games:
                p = get_plugin(gname)
                if p and p.config.get("enabled", True):
                    enabled.append(trigger)
            if enabled:
                rows.append(f"{cat_emoji} {cat_name}｜{chr(65295).join(enabled)}")
        if not rows:
            rows.append("暂无可用游戏")
        card = format_card(
            f"{site_name} · 游戏菜单",
            rows,
            footer="发送关键词即可参与，祝好运！",
            emoji="\U0001f30c",
        )
        self.record_play(ctx.user_id)
        return GameResponse.quick(f"@{ctx.username}\n{card}")
