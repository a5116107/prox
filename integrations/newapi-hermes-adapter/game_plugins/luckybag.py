"""Lucky Bag (福袋) - daily free open, random quality tiers. Card-style.

Complements checkin: checkin = fixed reward, luckybag = random surprise.
Runs entirely adapter-side, grants via reward.grant.small from activity pool.
"""

import random, time
from .base import GamePlugin, GameContext, GameResponse, format_card

# (emoji, name, weight, min_quota, max_quota, flavor)
LUCKY_TIERS = [
    ("🌿", "普通福袋", 50, 5000, 15000, "小有收获，积少成多"),
    ("✨", "小吉福袋", 25, 20000, 35000, "运气不错，继续加油"),
    ("🌸", "中吉福袋", 15, 40000, 60000, "今日有福，好事将近"),
    ("💎", "大吉福袋", 7, 65000, 85000, "财运亨通，把握机会"),
    ("🏆", "锦鲤福袋", 3, 90000, 125000, "鸿运当头！今日锦鲤附体"),
]


class LuckyBagGame(GamePlugin):
    name = "luckybag"
    display_name = "每日福袋"
    description = "每天免费拆一个福袋，随机惊喜奖励"
    tier = "daily"
    triggers = [
        "福袋",
        "每日福袋",
        "每天福袋",
        "开福袋",
        "拆福袋",
        "luckybag",
        "领福袋",
    ]
    default_config = {
        "enabled": True,
        "max_per_user_day": 1,
        "cooldown_seconds": 0,
        "budget_pool": "activity",
    }

    def handle(self, ctx, sm, budget, escrow):
        tiers = LUCKY_TIERS
        weights = [t[2] for t in tiers]
        idx = random.choices(range(len(tiers)), weights=weights, k=1)[0]
        emoji, name, _w, qmin, qmax, flavor = tiers[idx]
        reward_q = random.randint(qmin, qmax)
        reward_usd = budget.quota_to_usd(reward_q)

        budget.deduct(ctx.user_id, reward_q, self.config["budget_pool"])
        actions = [
            {
                "type": "reward.grant.small",
                "target_type": "user",
                "user_id": ctx.new_api_user_id,
                "quota_amount": reward_q,
                "budget_pool": self.config["budget_pool"],
                "reason": "luckybag_daily",
            }
        ]

        card = format_card(
            f"{name} · 已拆开",
            [
                ("玩家", f"@{ctx.username}"),
                ("品质", f"{emoji} {name}"),
                ("获得", f"+${reward_usd:.2f}"),
            ],
            footer=flavor + " · 明天再来拆！",
            emoji="\U0001f380",
        )
        self.record_play(ctx.user_id)
        return GameResponse(
            reply=f"@{ctx.username}\n{card}", actions=actions, event="luckybag"
        )
