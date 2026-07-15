"""Prediction Market (竞猜预测) - Binary outcome betting with weighted payouts."""

import time, random
from .base import GamePlugin, GameContext, GameResponse


class PredictGame(GamePlugin):
    name = "predict"
    display_name = "竞猜预测"
    description = "管理员出题，群友下注竞猜胜负"
    tier = "interactive"
    triggers = ["竞猜", "预测", "predict", "押注"]
    group_required = True
    default_config = {
        "enabled": True,
        "min_bet_usd": 0.01,
        "max_bet_usd": 0.50,
        "system_fee_pct": 0.15,
        "max_per_user_day": 30,
        "cooldown_seconds": 0,
        "budget_pool": "activity",
        "default_duration_sec": 300,
    }

    def __init__(self):
        super().__init__()
        self._markets = {}

    def handle(self, ctx, sm, budget, escrow):
        t = ctx.text.strip().lower()
        gid = ctx.group_id
        uid = ctx.user_id
        un = ctx.username
        mkt = self._markets.get(gid)

        if ctx.is_admin and t.startswith("竞猜 "):
            rest = t[3:].strip()
            parts = rest.rsplit(" ", 1)
            question = parts[0] if len(parts) > 1 else rest
            dur = self.config["default_duration_sec"]
            if len(parts) > 1:
                try:
                    d = int(parts[-1])
                    if d > 0:
                        dur = d
                        question = " ".join(parts[:-1])
                except:
                    pass

            self._markets[gid] = {
                "question": question,
                "bets_a": {},
                "bets_b": {},
                "total_a": 0,
                "total_b": 0,
                "started_at": time.time(),
                "duration": dur,
                "resolved": False,
                "winner": None,
            }
            min_s = "{:.0f}".format(dur // 60) if dur >= 60 else "{:.0f}s".format(dur)
            msg = (
                f"@{un} 📊 **竞猜开盘！**\n"
                f"📝 {question}\n"
                f"⏰ {min_s} 后截止\n\n"
                f"💡 下注: `押 A [金额]` 或 `押 B [金额]`\n"
                f"  例: `押 A 0.10`"
            )
            return GameResponse(reply=msg, event="predict_open")

        if (
            mkt
            and ("竞猜" in t or "状态" in t or "status" in t)
            and not any(k in t for k in ["押", "bet", "a ", "b "])
        ):
            elapsed = time.time() - mkt["started_at"]
            remaining = max(0, mkt["duration"] - elapsed)
            min_s = (
                "{:.0f}分".format(remaining // 60)
                if remaining >= 60
                else "{:.0f}秒".format(remaining)
            )

            if mkt["resolved"]:
                return GameResponse.quick(
                    f"📊 已结束: {mkt['question']}\n结果: {mkt['winner']}"
                )

            ta_usd = budget.quota_to_usd(mkt["total_a"])
            tb_usd = budget.quota_to_usd(mkt["total_b"])
            n_a = len(mkt["bets_a"])
            n_b = len(mkt["bets_b"])
            return GameResponse.quick(
                f"📊 **当前竞猜**\n📝 {mkt['question']}\n"
                f"⏰ 剩余 {min_s}\n"
                f"A. 赞成 | 💰 ${ta_usd:.2f} | 👥 {n_a}人\n"
                f"B. 反对 | 💰 ${tb_usd:.2f} | 👥 {n_b}人"
            )

        if mkt and not mkt["resolved"]:
            elapsed = time.time() - mkt["started_at"]
            if elapsed > mkt["duration"]:
                return GameResponse.quick(f"@{un} 竞猜已截止！")

            side = None
            bet_usd = self.config["min_bet_usd"]

            words = t.split()
            for i, w in enumerate(words):
                if w in ("押", "bet", "下注"):
                    if i + 1 < len(words):
                        side_raw = words[i + 1].upper()
                        side = {"A": "a", "B": "b"}.get(side_raw)
                try:
                    v = float(w)
                    if v > 0:
                        bet_usd = v
                except:
                    pass

            if not side or side not in ("a", "b"):
                return GameResponse.quick(f"@{un} 格式: `押 A [金额]` 或 `押 B [金额]`")

            bet_usd = max(
                self.config["min_bet_usd"], min(self.config["max_bet_usd"], bet_usd)
            )
            bet_q = budget.usd_to_quota(bet_usd)

            ok, why = self.can_play(ctx, budget)
            if not ok:
                return GameResponse.quick(f"@{un} {why}")

            budget.deduct(uid, bet_q, self.config["budget_pool"])

            # One active bet per user.  Keep external ids for UX/dedup, but
            # store New API ids for authoritative settlement.
            if uid in mkt["bets_a"]:
                mkt["total_a"] -= int(
                    (mkt["bets_a"].pop(uid) or {}).get("amount", 0) or 0
                )
            if uid in mkt["bets_b"]:
                mkt["total_b"] -= int(
                    (mkt["bets_b"].pop(uid) or {}).get("amount", 0) or 0
                )
            entry = {
                "amount": bet_q,
                "napi": int(ctx.new_api_user_id or 0),
                "username": un,
            }
            if side == "a":
                mkt["bets_a"][uid] = entry
                mkt["total_a"] += bet_q
            else:
                mkt["bets_b"][uid] = entry
                mkt["total_b"] += bet_q

            self.record_play(uid)
            side_label = "A. 赞成" if side == "a" else "B. 反对"
            return GameResponse(
                reply=f"@{un} ✅ 已下注 {side_label} ${bet_usd:.2f}！\n💡 输入 `竞猜状态` 查看实时赔率",
                actions=[
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ctx.new_api_user_id,
                        "quota_amount": -bet_q,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "predict_bet",
                    }
                ],
                event="predict_bet",
            )

        if ctx.is_admin and mkt and t.startswith("竞猜结算"):
            side = None
            words = t.split()
            for w in words[1:]:
                w = w.upper()
                if w in ("A", "B"):
                    side = w.lower()
                    break

            if not side:
                return GameResponse.quick(f"@{un} 格式: `竞猜结算 A` 或 `竞猜结算 B`")

            mkt["resolved"] = True
            mkt["winner"] = side

            win_bets = mkt["bets_a"] if side == "a" else mkt["bets_b"]
            lose_total = mkt["total_b"] if side == "a" else mkt["total_a"]
            win_total = mkt["total_a"] if side == "a" else mkt["total_b"]

            fee = int(lose_total * self.config["system_fee_pct"])
            prize_pool = lose_total - fee + win_total

            actions = []
            msg_parts = [
                f"@{un} 📊 **竞猜结算！**\n📝 {mkt['question']}\n结果: {'A. 赞成' if side == 'a' else 'B. 反对'}"
            ]

            if win_total == 0:
                msg_parts.append("\n⚠ 无人押中，退还所有赌注")
            else:
                for w_uid, entry in win_bets.items():
                    w_q = int((entry or {}).get("amount", 0) or 0)
                    share = int(prize_pool * w_q / win_total)
                    profit = share - w_q
                    winner_napi = int((entry or {}).get("napi") or 0)
                    winner_name = (entry or {}).get("username") or w_uid
                    actions.append(
                        {
                            "type": "reward.grant.small",
                            "target_type": "user",
                            "user_id": winner_napi,
                            "quota_amount": share,
                            "budget_pool": self.config["budget_pool"],
                            "reason": "predict_win",
                        }
                    )
                    msg_parts.append(
                        f"🏆 @{winner_name} +${budget.quota_to_usd(profit):.2f}"
                    )

            return GameResponse(
                reply="\n".join(msg_parts),
                actions=actions,
                event="predict_resolve",
            )

        return GameResponse.quick(f"@{un} 当前没有竞猜，管理员可发起: `竞猜 [问题]`")
