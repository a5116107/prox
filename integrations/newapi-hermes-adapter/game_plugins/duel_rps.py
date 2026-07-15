"""石头剪刀布对决 — 群聊版，双方随机出招，赌注正确扣除"""

import random, time
from .base import GamePlugin, GameContext, GameResponse

MOVES = ["石头", "剪刀", "布"]
BEATS = {"石头": "剪刀", "剪刀": "布", "布": "石头"}
EMOJI = {"石头": "✊", "剪刀": "✌️", "布": "🖐️"}


class DuelRPSGame(GamePlugin):
    name = "duel_rps"
    display_name = "石头剪刀布"
    description = "群聊1v1猜拳对决"
    tier = "pvp"
    triggers = ["猜拳", "石头剪刀布", "rps", "剪刀石头布"]
    group_required = True
    default_config = {
        "enabled": True,
        "min_stake_usd": 0.05,
        "max_stake_usd": 2.00,
        "system_fee_pct": 0.15,
        "timeout_seconds": 60,
        "max_per_user_day": 20,
        "cooldown_seconds": 5,
        "budget_pool": "game",
    }

    def __init__(self):
        super().__init__()
        self._challenges = {}

    def handle(self, ctx, sm, budget, escrow):
        t = ctx.text.strip()
        tl = t.lower()
        gid = ctx.group_id
        uid = ctx.user_id
        un = ctx.username

        if tl in ("接受", "应战", "接受挑战", "accept"):
            return self._accept(gid, uid, un, ctx, budget)

        for trigger in self.triggers:
            if tl.startswith(trigger):
                return self._initiate(t, gid, uid, un, ctx, budget)

        return None

    def _initiate(self, t, gid, uid, un, ctx, budget):
        parts = t.split()
        target = None
        stake_usd = None

        for p in parts[1:]:
            p_clean = p.strip()
            if p_clean.startswith("@"):
                target = p_clean[1:]
            else:
                try:
                    v = float(p_clean.replace("$", ""))
                    if v > 0:
                        stake_usd = v
                except:
                    pass

        min_s = self.config["min_stake_usd"]
        max_s = self.config["max_stake_usd"]

        if stake_usd is None:
            return GameResponse.quick(
                f"@{un} ✊✌️🖐️ **猜拳对决**\n\n"
                f"格式: `猜拳 @对手 金额`\n"
                f"例: `猜拳 @张三 0.10`\n"
                f"金额: ${min_s:.2f} ~ ${max_s:.2f}\n"
                f"赢家拿 {int((1 - self.config['system_fee_pct']) * 100)}%，手续费 {int(self.config['system_fee_pct'] * 100)}%"
            )

        stake_usd = max(min_s, min(max_s, stake_usd))
        stake_q = budget.usd_to_quota(stake_usd)

        ok, why = self.can_play(ctx, budget)
        if not ok:
            return GameResponse.quick(f"@{un} {why}")

        ok, why = budget.check_budget(self.config["budget_pool"], stake_q)
        if not ok:
            return GameResponse.quick(f"@{un} 游戏预算已用完，明天再来")

        budget.deduct(uid, stake_q, self.config["budget_pool"])

        self._challenges[gid] = {
            "challenger_id": uid,
            "challenger_name": un,
            "challenger_napi": ctx.new_api_user_id,
            "target": target,
            "stake": stake_q,
            "timeout": time.time() + self.config["timeout_seconds"],
        }
        self.record_play(uid)

        target_text = f"@{target}" if target else "所有人"
        return GameResponse(
            reply=(
                f"@{un} ✊✌️🖐️ 发起猜拳挑战！\n\n"
                f"🎯 对手: {target_text}\n"
                f"💰 赌注: ${stake_usd:.2f}/人\n"
                f"⏰ {self.config['timeout_seconds']}秒内回复 `接受` 应战\n\n"
                f"⚡ 接受后双方随机出招，即时开奖！"
            ),
            actions=[
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": ctx.new_api_user_id,
                    "quota_amount": -stake_q,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "duel_rps_stake",
                }
            ],
            event="duel_rps_init",
        )

    def _accept(self, gid, uid, un, ctx, budget):
        ch = self._challenges.get(gid)
        if not ch or time.time() > ch["timeout"]:
            if gid in self._challenges:
                del self._challenges[gid]
            return GameResponse.quick(
                f"@{un} 当前没有待接受的猜拳挑战，先发送 `猜拳 @对手 0.10` 开局"
            )

        if ch["target"] and ch["target"] != uid and ch["target"] != un:
            return GameResponse.quick(f"@{un} 这场猜拳挑战是发给 @{ch['target']} 的")
        if ch["challenger_id"] == uid:
            return GameResponse.quick(f"@{un} 不能接受自己的挑战")

        stake = ch["stake"]

        ok, why = self.can_play(ctx, budget)
        if not ok:
            return GameResponse.quick(f"@{un} {why}")

        ok, why = budget.check_budget(self.config["budget_pool"], stake)
        if not ok:
            del self._challenges[gid]
            budget.deduct(ch["challenger_id"], -stake, self.config["budget_pool"])
            return GameResponse(
                reply=f"@{un} 预算不足，挑战取消。@{ch['challenger_name']} 赌注已退还",
                actions=[
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ch["challenger_napi"],
                        "quota_amount": stake,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "duel_rps_refund",
                    }
                ],
                event="duel_rps_cancel",
            )

        budget.deduct(uid, stake, self.config["budget_pool"])
        self.record_play(uid)
        del self._challenges[gid]

        # 应战方真实扣注金（发起方已在 _initiate 扣）
        acceptor_stake = [
            {
                "type": "reward.grant.small",
                "target_type": "user",
                "user_id": ctx.new_api_user_id,
                "quota_amount": -stake,
                "budget_pool": self.config["budget_pool"],
                "reason": "duel_rps_stake",
            }
        ]

        m1 = random.choice(MOVES)
        m2 = random.choice(MOVES)
        total = stake * 2
        fee = int(total * self.config["system_fee_pct"])
        prize = total - fee

        if m1 == m2:
            return GameResponse(
                reply=(
                    f"✊✌️🖐️ **猜拳对决！**\n\n"
                    f"{EMOJI[m1]} @{ch['challenger_name']} 出了 {m1}\n"
                    f"{EMOJI[m2]} @{un} 出了 {m2}\n\n"
                    f"🤝 **平局！** 各退回 ${budget.quota_to_usd(stake):.2f}"
                ),
                actions=acceptor_stake
                + [
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ch["challenger_napi"],
                        "quota_amount": stake,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "duel_rps_refund",
                    },
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ctx.new_api_user_id,
                        "quota_amount": stake,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "duel_rps_refund",
                    },
                ],
                event="duel_rps_tie",
            )

        if BEATS[m1] == m2:
            winner_napi = ch["challenger_napi"]
            winner_name = ch["challenger_name"]
        else:
            winner_napi = ctx.new_api_user_id
            winner_name = un

        budget.record_income(fee, self.config["budget_pool"])
        return GameResponse(
            reply=(
                f"✊✌️🖐️ **猜拳对决！**\n\n"
                f"{EMOJI[m1]} @{ch['challenger_name']} 出了 {m1}\n"
                f"{EMOJI[m2]} @{un} 出了 {m2}\n\n"
                f"🏆 @{winner_name} 获胜！+${budget.quota_to_usd(prize):.2f}\n"
                f"💸 手续费 ${budget.quota_to_usd(fee):.2f}"
            ),
            actions=acceptor_stake
            + [
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": winner_napi,
                    "quota_amount": prize,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "duel_rps_win",
                },
            ],
            event="duel_rps_result",
        )
