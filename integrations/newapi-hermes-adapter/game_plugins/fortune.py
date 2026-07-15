"""Fortune Telling (运势抽签) - Daily system-sitting game."""

import random, time
from datetime import datetime
from .base import GamePlugin, GameContext, GameResponse

FORTUNES = [
    ("大吉", "诸事顺遂，财源广进", "💎"),
    ("吉", "运势平稳，小有收获", "✨"),
    ("中吉", "好事多磨，耐心等待", "🌸"),
    ("小吉", "略有波折，注意沟通", "🌤"),
    ("末吉", "凡事谨慎，勿贪小利", "⚠"),
    ("凶", "今日低调，守住本金", "🌧"),
]


class FortuneGame(GamePlugin):
    name = "fortune"
    display_name = "运势抽签"
    description = "每日一签，测试今日运气"
    tier = "daily"
    triggers = ["运势", "抽签", "占卜", "fortune"]
    default_config = {
        "enabled": True,
        "max_per_user_day": 1,
        "cooldown_seconds": 0,
        "budget_pool": "game",
        # Lucky users get a tiny bonus
        "lucky_bonus_quota": 50000,  # $0.10
        "lucky_chance": 0.15,
    }

    def handle(self, ctx, sm, budget, escrow):
        today = datetime.now().strftime("%Y-%m-%d")
        fortune, desc, emoji = random.choice(FORTUNES)
        msg = f"@{ctx.username} {emoji} **{fortune}**\n📜 {desc}\n🕐 {today}"

        bonus_q = 0
        actions = []
        if fortune == "大吉" and random.random() < self.config["lucky_chance"]:
            bonus_q = self.config["lucky_bonus_quota"]
            bonus_usd = budget.quota_to_usd(bonus_q)
            msg += f"\n\n🎁 大吉彩头 +${bonus_usd:.2f}"
            budget.deduct(ctx.user_id, bonus_q, self.config["budget_pool"])
            actions.append(
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": ctx.new_api_user_id,
                    "quota_amount": bonus_q,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "fortune_lucky",
                }
            )
        msg += "\n💡 明天再来抽一签！"

        self.record_play(ctx.user_id)
        return GameResponse(reply=msg, actions=actions, event="fortune")
