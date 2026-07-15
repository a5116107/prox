"""Dice Game (骰子) - Roll dice, bet on outcomes. Fixed triple detection + adjusted payouts."""

import random, time
from .base import GamePlugin, GameContext, GameResponse


class DiceGame(GamePlugin):
    name = "dice"
    display_name = "摇骰子"
    description = "骰子比大小/单双/豹子，最低 $0.01/注"
    tier = "interactive"
    triggers = ["骰子", "摇骰子", "dice", "掷骰"]
    default_config = {
        "enabled": True,
        "min_bet_quota": 5000,
        "max_bet_quota": 500000,
        "max_per_user_day": 50,
        "cooldown_seconds": 3,
        "budget_pool": "game",
        "system_fee_pct": 0.05,
    }

    BET_ALIASES = {
        "big": "big",
        "大": "big",
        "small": "small",
        "小": "small",
        "odd": "odd",
        "单": "odd",
        "even": "even",
        "双": "even",
        "triple": "triple",
        "豹子": "triple",
    }

    BET_INFO = {
        "big": {"desc": "大 (11-17)", "payout": 1.9},
        "small": {"desc": "小 (4-10)", "payout": 1.9},
        "odd": {"desc": "单", "payout": 1.9},
        "even": {"desc": "双", "payout": 1.9},
        "triple": {"desc": "豹子 (三同)", "payout": 30.0},
    }

    def _check_bet(self, bet_type, d1, d2, d3):
        total = d1 + d2 + d3
        if bet_type == "big":
            return 11 <= total <= 17 and not (d1 == d2 == d3)
        elif bet_type == "small":
            return 4 <= total <= 10 and not (d1 == d2 == d3)
        elif bet_type == "odd":
            return total % 2 == 1
        elif bet_type == "even":
            return total % 2 == 0
        elif bet_type == "triple":
            return d1 == d2 == d3
        return False

    def handle(self, ctx, sm, budget, escrow):
        t = ctx.text.strip().lower()
        parts = t.split()

        bet_type = None
        bet_usd = 0.01

        for part in parts[1:]:
            p = part.strip()
            if p in self.BET_ALIASES:
                bet_type = self.BET_ALIASES[p]
            else:
                try:
                    v = float(p)
                    if v > 0:
                        bet_usd = v
                except:
                    pass

        if not bet_type:
            d1, d2, d3 = [random.randint(1, 6) for _ in range(3)]
            total = d1 + d2 + d3
            cls = "大" if total >= 11 else "小"
            odd = "单" if total % 2 == 1 else "双"
            triple = " 豹子!" if d1 == d2 == d3 else ""
            msg = (
                f"@{ctx.username} 🎲 {d1} | {d2} | {d3}\n"
                f"合计: {total} ({cls}/{odd}){triple}\n"
                f"💡 下注: `骰子 大/小/单/双/豹子 金额`"
            )
            self.record_play(ctx.user_id)
            return GameResponse(reply=msg, event="dice_roll")

        bet_q = budget.usd_to_quota(bet_usd)
        min_q = self.config["min_bet_quota"]
        max_q = self.config["max_bet_quota"]

        if bet_q < min_q:
            return GameResponse.quick(
                f"@{ctx.username} 最低下注 ${budget.quota_to_usd(min_q):.2f}"
            )
        if bet_q > max_q:
            return GameResponse.quick(
                f"@{ctx.username} 最高下注 ${budget.quota_to_usd(max_q):.2f}"
            )

        ok, why = self.can_play(ctx, budget)
        if not ok:
            return GameResponse.quick(f"@{ctx.username} {why}")

        budget.deduct(ctx.user_id, bet_q, self.config["budget_pool"])

        d1, d2, d3 = [random.randint(1, 6) for _ in range(3)]
        total = d1 + d2 + d3
        dice_str = f"🎲 {d1} | {d2} | {d3} = **{total}**"

        info = self.BET_INFO[bet_type]
        won = self._check_bet(bet_type, d1, d2, d3)

        self.record_play(ctx.user_id)

        if won:
            fee_pct = self.config["system_fee_pct"]
            gross = int(bet_q * info["payout"])
            fee_q = int(gross * fee_pct)
            win_q = gross - fee_q
            budget.record_income(fee_q, self.config["budget_pool"])

            return GameResponse(
                reply=(
                    f"@{ctx.username} {dice_str}\n"
                    f"🎉 押 {info['desc']} **赢了！** +${budget.quota_to_usd(win_q):.2f}\n"
                    f"  (赔率 {info['payout']}x, 手续费 {fee_pct * 100:.0f}%)"
                ),
                actions=[
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ctx.new_api_user_id,
                        "quota_amount": -bet_q,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "dice_bet",
                    },
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ctx.new_api_user_id,
                        "quota_amount": win_q,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "dice_win",
                    },
                ],
                event="dice_win",
            )
        else:
            budget.record_income(bet_q, self.config["budget_pool"])
            return GameResponse(
                reply=f"@{ctx.username} {dice_str}\n😔 押 {info['desc']} 输了 -${bet_usd:.2f}",
                actions=[
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ctx.new_api_user_id,
                        "quota_amount": -bet_q,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "dice_bet",
                    },
                ],
                event="dice_lose",
            )
