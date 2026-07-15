"""Bounty System (悬赏) - Post tasks with rewards, 24h timeout + auto-refund."""

import time, random
from .base import GamePlugin, GameContext, GameResponse


class BountyGame(GamePlugin):
    name = "bounty"
    display_name = "悬赏任务"
    description = "发布悬赏任务，他人接单完成获奖励"
    tier = "interactive"
    triggers = ["悬赏", "bounty", "发布悬赏", "接单", "完成悬赏", "取消悬赏"]
    default_config = {
        "enabled": True,
        "min_bounty_usd": 0.10,
        "max_bounty_usd": 2.00,
        "platform_fee_pct": 0.15,
        "max_per_user_day": 20,
        "cooldown_seconds": 0,
        "budget_pool": "activity",
        "timeout_hours": 24,
    }

    def __init__(self):
        super().__init__()
        self._bounties = {}
        self._counter = 0
        try:
            from state_store import get_store

            self._store = get_store()
            self._load_state()
        except Exception:
            self._store = None

    def _load_state(self):
        if not self._store:
            return
        saved = self._store.get("bounty", "bounties", {})
        self._counter = self._store.get("bounty", "counter", 0)
        now = time.time()
        timeout_sec = self.config.get("timeout_hours", 24) * 3600
        for bid_str, b in saved.items():
            if b["status"] == "completed":
                continue
            if now - b.get("created_at", 0) > timeout_sec and b["status"] in (
                "open",
                "claimed",
            ):
                b["status"] = "expired"
                continue
            self._bounties[int(bid_str)] = b

    def _save_state(self):
        if not self._store:
            return
        self._store.set(
            "bounty", "bounties", {str(k): v for k, v in self._bounties.items()}
        )
        self._store.set("bounty", "counter", self._counter)
        self._store.save("bounty")

    def _cleanup_expired(self, budget):
        now = time.time()
        timeout_sec = self.config.get("timeout_hours", 24) * 3600
        refunds = []
        for bid in list(self._bounties.keys()):
            b = self._bounties[bid]
            if (
                b["status"] in ("open", "claimed")
                and now - b.get("created_at", 0) > timeout_sec
            ):
                b["status"] = "expired"
                refunds.append(
                    {
                        "bid": bid,
                        "poster": b["poster"],
                        "poster_napi": b.get("poster_napi", 0),
                        "amount": b["reward"] + b["fee"],
                        "poster_name": b["poster_name"],
                    }
                )
        if refunds:
            self._save_state()
        return refunds

    def handle(self, ctx, sm, budget, escrow):
        t = ctx.text.strip().lower()
        gid = ctx.group_id
        uid = ctx.user_id
        un = ctx.username

        expired_refunds = self._cleanup_expired(budget)
        refund_actions = []
        refund_msgs = []
        for r in expired_refunds:
            refund_actions.append(
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": r["poster_napi"],
                    "quota_amount": r["amount"],
                    "budget_pool": self.config["budget_pool"],
                    "reason": f"bounty_{r['bid']}_expired_refund",
                }
            )
            refund_msgs.append(
                f"⏰ 悬赏 #{r['bid']} 已过期，已退还给 @{r['poster_name']}"
            )

        if t in ("悬赏", "bounty", "悬赏列表", "bountylist"):
            active = {
                bid: b
                for bid, b in self._bounties.items()
                if b["status"] in ("open", "claimed")
            }
            if not active and not refund_msgs:
                return GameResponse.quick(
                    f"@{un} 当前没有悬赏任务\n💡 发布: `发布悬赏 [金额] [描述]`"
                )

            lines = [f"@{un} 📋 **悬赏列表**"]
            for bid, b in sorted(active.items()):
                status = "⏳" if b["status"] == "open" else "🔒"
                age = int((time.time() - b.get("created_at", 0)) / 3600)
                lines.append(
                    f"{status} #{bid} ${budget.quota_to_usd(b['reward']):.2f} - {b['desc'][:20]} ({age}h前)"
                )
            lines.extend(refund_msgs)
            return GameResponse(
                reply="\n".join(lines), actions=refund_actions, event="bounty_list"
            )

        if "发布" in t or "create" in t:
            parts = ctx.text.strip().split(None, 2)
            usd = self.config["min_bounty_usd"]
            desc = "悬赏任务"

            for p in parts[1:]:
                try:
                    v = float(p.replace("$", ""))
                    if v > 0:
                        usd = v
                except:
                    if len(p) > 1:
                        desc = p

            usd = max(
                self.config["min_bounty_usd"], min(self.config["max_bounty_usd"], usd)
            )
            q = budget.usd_to_quota(usd)
            fee = int(q * self.config["platform_fee_pct"])
            budget.record_income(fee, self.config["budget_pool"])
            total = q + fee

            ok, why = budget.check_user_limit(uid, total, self.config["budget_pool"])
            if not ok:
                return GameResponse.quick(f"@{un} 今日额度不足")

            budget.deduct(uid, total, self.config["budget_pool"])

            self._counter += 1
            bid = self._counter
            timeout_h = self.config.get("timeout_hours", 24)
            self._bounties[bid] = {
                "id": bid,
                "poster": uid,
                "poster_name": un,
                "poster_napi": ctx.new_api_user_id,
                "desc": desc,
                "reward": q,
                "fee": fee,
                "status": "open",
                "claimer": None,
                "created_at": time.time(),
            }
            self._save_state()
            self.record_play(uid)

            actions = refund_actions + [
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": ctx.new_api_user_id,
                    "quota_amount": -total,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "bounty_create",
                }
            ]

            msg = (
                f"@{un} 📢 **悬赏发布！** #{bid}\n"
                f"📝 {desc}\n"
                f"💰 赏金: ${usd:.2f} (手续费 {int(self.config['platform_fee_pct'] * 100)}%)\n"
                f"⏰ {timeout_h}小时内无人完成自动退款\n"
                f"💡 接单: `接单 {bid}` | 完成: `完成悬赏 {bid}`"
            )
            return GameResponse(reply=msg, actions=actions, event="bounty_create")

        if "接单" in t or "claim" in t:
            bid = self._parse_bid(t)
            if not bid or bid not in self._bounties:
                return GameResponse.quick(f"@{un} 悬赏不存在，发送「悬赏」查看列表")
            b = self._bounties[bid]
            if b["status"] != "open":
                return GameResponse.quick(f"@{un} 悬赏 #{bid} 已被接单或已完成")
            if b["poster"] == uid:
                return GameResponse.quick(f"@{un} 不能接自己的悬赏")

            b["status"] = "claimed"
            b["claimer"] = uid
            b["claimer_name"] = un
            b["claimer_napi"] = ctx.new_api_user_id
            b["claimed_at"] = time.time()
            self._save_state()

            msg = (
                f"@{un} 🔒 已接单 #{bid}\n"
                f"📝 {b['desc']}\n"
                f"💰 赏金: ${budget.quota_to_usd(b['reward']):.2f}\n"
                f"💡 完成后由 @{b['poster_name']} 确认: `完成悬赏 {bid}`"
            )
            return GameResponse(reply=msg, actions=refund_actions, event="bounty_claim")

        if "完成" in t or "complete" in t or "done" in t:
            bid = self._parse_bid(t)
            if not bid or bid not in self._bounties:
                return GameResponse.quick(f"@{un} 悬赏不存在")
            b = self._bounties[bid]
            if b["poster"] != uid and not ctx.is_admin:
                return GameResponse.quick(f"@{un} 只有发布者可以确认完成")
            if b["status"] != "claimed":
                return GameResponse.quick(f"@{un} 悬赏 #{bid} 未被接单或已完成")

            b["status"] = "completed"
            b["completed_at"] = time.time()
            self._save_state()
            self.record_play(uid)

            actions = refund_actions + [
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": b["claimer_napi"],
                    "quota_amount": b["reward"],
                    "budget_pool": self.config["budget_pool"],
                    "reason": "bounty_complete",
                }
            ]

            return GameResponse(
                reply=(
                    f"@{un} ✅ **悬赏 #{bid} 已完成！**\n"
                    f"📝 {b['desc']}\n"
                    f"🏆 @{b['claimer_name']} +${budget.quota_to_usd(b['reward']):.2f}"
                ),
                actions=actions,
                event="bounty_complete",
            )

        if "取消" in t or "cancel" in t:
            bid = self._parse_bid(t)
            if not bid or bid not in self._bounties:
                return GameResponse.quick(f"@{un} 悬赏不存在")
            b = self._bounties[bid]
            if b["poster"] != uid and not ctx.is_admin:
                return GameResponse.quick(f"@{un} 只有发布者可以取消")
            if b["status"] not in ("open",):
                return GameResponse.quick(f"@{un} 悬赏 #{bid} 已被接单，无法取消")

            b["status"] = "cancelled"
            refund_total = b["reward"] + b["fee"]
            self._save_state()

            actions = refund_actions + [
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": b.get("poster_napi", 0),
                    "quota_amount": refund_total,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "bounty_cancel_refund",
                }
            ]
            return GameResponse(
                reply=f"@{un} ❌ 悬赏 #{bid} 已取消，赏金+手续费已退还",
                actions=actions,
                event="bounty_cancel",
            )

        return GameResponse.quick(
            f"@{un} 💡 悬赏系统:\n"
            f"  发布: `发布悬赏 [金额] [描述]`\n"
            f"  查看: `悬赏`  |  接单: `接单 编号`\n"
            f"  完成: `完成悬赏 编号`  |  取消: `取消悬赏 编号`\n"
            f"  ⏰ 超时{self.config.get('timeout_hours', 24)}小时自动退款"
        )

    def _parse_bid(self, t):
        for p in t.split():
            p = p.strip("#")
            try:
                return int(p)
            except:
                pass
        return None
