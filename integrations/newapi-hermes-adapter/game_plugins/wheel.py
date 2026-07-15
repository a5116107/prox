"""Wheel of Fortune (幸运转盘) - One spin per cooldown, random prizes."""

import random, time
from .base import GamePlugin, GameContext, GameResponse

WHEEL_SEGMENTS = [
    # (emoji, label, quota_amount)
    ("💰", "特等奖  $2.00", 1000000),
    ("🎁", "一等奖  $1.00", 500000),
    ("💎", "二等奖  $0.40", 200000),
    ("✨", "三等奖  $0.10", 50000),
    ("🌸", "四等奖  $0.02", 10000),
    ("🍀", "五等奖  $0.01", 5000),
    ("🌧", "谢谢参与", 0),
    ("😅", "再来一次", 0),
]
WEIGHTS = [1, 3, 5, 10, 15, 25, 25, 16]


class WheelGame(GamePlugin):
    name = "wheel"
    display_name = "幸运转盘"
    description = "转盘抽奖，每次 $0.01，特等奖 $2.00"
    tier = "interactive"
    triggers = ["转盘", "幸运转盘", "wheel", "抽奖", "轮盘"]
    default_config = {
        "enabled": True,
        "spin_cost_quota": 10000,  # $0.01
        "max_per_user_day": 10,
        "cooldown_seconds": 5,
        "budget_pool": "activity",
        "free_spins_per_day": 1,
    }
    _free_used = {}

    def _load_free(self):
        try:
            from state_store import get_store

            saved = get_store().get("wheel", "free_used", {})
            if saved:
                self.__class__._free_used = saved
        except Exception:
            pass

    def _save_free(self):
        try:
            from state_store import get_store

            get_store().set("wheel", "free_used", self._free_used)
            get_store().save("wheel")
        except Exception:
            pass

    def handle(self, ctx, sm, budget, escrow):
        if not hasattr(self, "_free_loaded"):
            self._load_free()
            self._free_loaded = True
        today = time.strftime("%Y-%m-%d")
        uid = ctx.user_id
        un = ctx.username
        cost = self.config["spin_cost_quota"]

        # Check free spins
        free_key = f"{uid}:{today}"
        free_used = self._free_used.get(free_key, 0)
        free_limit = self.config["free_spins_per_day"]
        is_free = free_used < free_limit

        if not is_free:
            ok, why = self.can_play(ctx, budget)
            if not ok:
                return GameResponse.quick(f"@{un} {why}")
            budget.deduct(uid, cost, self.config["budget_pool"])

        # Spin
        segments = list(zip(WHEEL_SEGMENTS, WEIGHTS))
        items, wts = zip(*segments)
        idx = random.choices(range(len(items)), weights=wts, k=1)[0]
        emoji, label, reward_q = items[idx]

        # Animate
        emojis = ["🔄", "🎡", "💫", "⭐", emoji]
        spin_anim = " → ".join(emojis)

        cost_usd = budget.quota_to_usd(cost)
        label_tag = "🎟 免费转" if is_free else f"💵 消耗 ${cost_usd:.2f}"

        msg = f"@{un} {spin_anim}\n{label_tag} → {emoji} **{label}**"

        actions = []
        if reward_q > 0:
            budget.deduct(uid, reward_q, self.config["budget_pool"])
            actions.append(
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": ctx.new_api_user_id,
                    "quota_amount": reward_q,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "wheel_prize",
                }
            )

        if is_free:
            self._free_used[free_key] = free_used + 1
            self._save_free()
            remaining = free_limit - free_used - 1
            msg += f"\n🎫 今日免费转: {remaining}/{free_limit} 次"
        else:
            self.record_play(uid)

        msg += f"\n💡 再转一次: `转盘` (每转 ${cost_usd:.2f})"
        return GameResponse(reply=msg, actions=actions, event="wheel_spin")
