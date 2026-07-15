"""Treasure Hunt - entry fee per round, timed draw or refund."""

import random
import time
from collections import defaultdict

from .base import GamePlugin, GameResponse


class TreasureGame(GamePlugin):
    name = "treasure"
    display_name = "夺宝奇兵"
    description = "入场费竞猜，5分钟/轮，前三瓜分奖池；人数不足自动退款"
    tier = "interactive"
    triggers = ["夺宝", "夺宝奇兵", "加入夺宝", "treasure"]
    group_required = True
    default_config = {
        "enabled": True,
        "round_duration_sec": 300,
        "min_players": 3,
        "max_tokens_per_user": 5,
        "entry_cost_quota": 500000,
        "prize_split": [50, 30, 20],
        "system_fee_pct": 0.10,
        "announce_times": [240, 180, 120, 60],
        "max_per_user_day": 99,
        "budget_pool": "game",
    }

    def __init__(self):
        super().__init__()
        self._ap = defaultdict(int)
        self._ar = defaultdict(int)
        self._last_round_end = {}

    def _session(self, gid, sm):
        if hasattr(sm, "get_session"):
            return sm.get_session(gid)
        return sm.get_active(gid)

    def _gs(self, gid, sm):
        a = self._session(gid, sm)
        if a and a.game_name == "treasure":
            return a.state
        s = sm.start(
            gid,
            "treasure",
            int(self.config["round_duration_sec"]),
            {
                "participants": {},
                "pool": self._ap.get(gid, 0),
                "acc_rounds": self._ar.get(gid, 0),
                "started_at": time.time(),
            },
        )
        return s.state

    def _usd(self, q, b):
        return f"${b.quota_to_usd(q):.2f}"

    def _mark_end(self, gid, sm):
        try:
            sm.end(gid)
        except Exception:
            pass
        self._last_round_end[gid] = time.time()

    def _join(self, ctx, state, budget):
        p = state["participants"]
        uid = str(ctx.user_id)
        un = ctx.username
        cost = int(self.config["entry_cost_quota"])
        mt = int(self.config["max_tokens_per_user"])
        if uid in p and int(p[uid].get("tokens") or 0) >= mt:
            return GameResponse.quick(
                f"@{un} 已达上限({mt}次)\n奖池:{self._usd(state['pool'], budget)}"
            )

        budget.deduct(uid, cost, self.config["budget_pool"])
        if uid not in p:
            p[uid] = {
                "username": un,
                "napi": int(ctx.new_api_user_id or 0),
                "tokens": 0,
                "spent": 0,
            }
        else:
            p[uid]["username"] = un
            p[uid]["napi"] = int(ctx.new_api_user_id or p[uid].get("napi") or 0)
        p[uid]["tokens"] = int(p[uid].get("tokens") or 0) + 1
        p[uid]["spent"] = int(p[uid].get("spent") or 0) + cost
        state["pool"] = int(state.get("pool") or 0) + cost

        tk = p[uid]["tokens"]
        tt = sum(int(x.get("tokens") or 0) for x in p.values())
        pc = len(p)
        cost_usd = budget.quota_to_usd(cost)
        return GameResponse(
            reply=(
                f"@{un} ✨ 加入成功！（{tk}/{mt}）\n"
                f"💸 消耗：${cost_usd:.2f}\n"
                f"🎫 令牌：{tk}个|💰 奖池:{self._usd(state['pool'], budget)}\n"
                f"👥 {pc}人参战|总令牌{tt}个\n"
                f"💡 多次加入提升中奖率，每人最多{mt}次！"
            ),
            actions=[
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": ctx.new_api_user_id,
                    "target_external_id": uid,
                    "quota_amount": -cost,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "treasure_entry",
                }
            ],
            event="treasure_join",
        )

    def _announce(self, state, budget, tl):
        p = state["participants"]
        pool_usd = budget.quota_to_usd(int(state.get("pool") or 0))
        pc = len(p)
        tt = sum(int(x.get("tokens") or 0) for x in p.values())
        m = int(tl // 60)
        s = int(tl % 60)
        msg = f"📢 当前进展\n💰 奖池：{pool_usd:.2f}$ 👥 {pc}人 🎫 {tt}票\n⏰ 剩余：{m}分{s}秒\n\n"
        if pc < int(self.config["min_players"]):
            msg += f"⚠️ 还需{int(self.config['min_players']) - pc}人开奖；人数不足将自动退款。\n"
        else:
            msg += "🔥 已满足开奖人数，即将揭晓！\n"
        msg += "\n发送「加入夺宝」参与！"
        return msg

    def _draw(self, state, budget):
        p = state["participants"]
        pool = int(state.get("pool") or 0)
        sfee = int(pool * float(self.config["system_fee_pct"]))
        prize_pool = max(pool - sfee, 0)
        if sfee > 0:
            budget.record_income(sfee, self.config["budget_pool"])

        entries = []
        for external_uid, x in p.items():
            tokens = int(x.get("tokens") or 0)
            entries.extend(
                [
                    (
                        external_uid,
                        int(x.get("napi") or 0),
                        x.get("username") or external_uid,
                    )
                ]
                * tokens
            )
        random.shuffle(entries)

        winners = []
        seen = set()
        for external_uid, napi, un in entries:
            if external_uid not in seen:
                winners.append({"uid": napi, "external_uid": external_uid, "un": un})
                seen.add(external_uid)
            if len(winners) >= 3:
                break

        split = list(self.config["prize_split"])
        payouts = [
            {**w, "amt": int(prize_pool * split[i] / 100)}
            for i, w in enumerate(winners[:3])
            if int(w.get("uid") or 0) > 0
        ]
        em = ["🥇", "🥈", "🥉"]
        lines = ["🎊 夺宝奇兵开奖结果！\n"]
        for i, p2 in enumerate(payouts[:3]):
            lines.append(f"{em[i]} @{p2['un']} +{budget.quota_to_usd(p2['amt']):.2f}$")
        lines.append(f"\n💼 平台基金抽佣：{budget.quota_to_usd(sfee):.2f}$")
        lines.append("💰 奖金已发放～")
        return payouts, "\n".join(lines)

    def _refund_insufficient(self, ctx, sm, state, budget, end_session=True):
        gid = ctx.group_id
        p = state.get("participants") or {}
        actions = []
        refund_total = 0
        for external_uid, x in p.items():
            refund = int(x.get("spent") or 0)
            napi = int(x.get("napi") or 0)
            if refund > 0 and napi > 0:
                refund_total += refund
                actions.append(
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": napi,
                        "target_external_id": str(external_uid),
                        "quota_amount": refund,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "treasure_refund",
                        "source_type": "game_refund",
                    }
                )
        self._ap[gid] = 0
        self._ar[gid] = 0
        if end_session:
            self._mark_end(gid, sm)
        else:
            self._last_round_end[gid] = time.time()
        reply = (
            "📣 夺宝奇兵本轮结束：人数不足，已自动退款\n"
            f"👥 {len(p)}人/需{int(self.config['min_players'])}人\n"
            f"💸 退款合计：{budget.quota_to_usd(refund_total):.2f}$\n"
            "发送「加入夺宝」可参与下一轮。"
        )
        return GameResponse(reply=reply, actions=actions, event="treasure_refund")

    def _finalize(self, ctx, sm, state, budget, end_session=True):
        p = state.get("participants") or {}
        gid = ctx.group_id
        if len(p) >= int(self.config["min_players"]):
            payouts, ann = self._draw(state, budget)
            self._ap[gid] = 0
            self._ar[gid] = 0
            if end_session:
                self._mark_end(gid, sm)
            else:
                self._last_round_end[gid] = time.time()
            acts = [
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": pp["uid"],
                    "target_external_id": str(pp.get("external_uid") or ""),
                    "quota_amount": pp["amt"],
                    "budget_pool": self.config["budget_pool"],
                    "reason": f"treasure_win_{i + 1}",
                }
                for i, pp in enumerate(payouts[:3])
                if int(pp.get("uid") or 0) > 0 and int(pp.get("amt") or 0) > 0
            ]
            return GameResponse(reply=ann, actions=acts, event="treasure_draw")
        return self._refund_insufficient(
            ctx, sm, state, budget, end_session=end_session
        )

    def handle(self, ctx, sm, budget, escrow):
        raw_text = (ctx.text or "").strip().lower()
        join_words = [
            "加入夺宝",
            "参加夺宝",
            "报名夺宝",
            "join treasure",
            "treasure join",
        ]
        info_prefixes = ["夺宝", "夺宝奇兵", "treasure"]

        sess = self._session(ctx.group_id, sm)
        if sess and sess.game_name == "treasure" and sess.is_expired():
            return self._finalize(ctx, sm, sess.state or {}, budget)

        if any(raw_text.startswith(p) for p in info_prefixes) and not any(
            w in raw_text for w in join_words
        ):
            if sess and sess.game_name == "treasure" and getattr(sess, "state", None):
                st = sess.state
                pool = int(st.get("pool") or 0)
                participants = st.get("participants", {})
                return GameResponse.quick(
                    f"📢 当前夺宝进展\n💰 奖池：{budget.quota_to_usd(pool):.2f}$ 👥 {len(participants)}人\n发送「加入夺宝」确认消耗额度参与。"
                )
            return GameResponse.quick(
                "💎 夺宝奇兵：发送「加入夺宝」将消耗额度参与；发送「夺宝 / 夺宝奇兵」查看当前进展或规则，不会扣款。"
            )

        state = self._gs(ctx.group_id, sm)
        elapsed = time.time() - float(state.get("started_at") or time.time())
        if elapsed >= int(self.config["round_duration_sec"]) or raw_text in (
            "__auto_draw__",
            "__auto_close__",
            "__auto_expire__",
        ):
            return self._finalize(ctx, sm, state, budget)

        if any(w in raw_text for w in join_words):
            return self._join(ctx, state, budget)
        return None

    def _check_announce(self, gid, sm, budget):
        a = sm.get_active(gid)
        if not a or a.game_name != "treasure":
            return ""
        state = a.state
        elapsed = time.time() - float(state.get("started_at") or a.started_at)
        tl = max(0, int(int(self.config["round_duration_sec"]) - elapsed))
        for at in sorted(self.config["announce_times"], reverse=True):
            if tl <= int(at) and state.get("ann_" + str(at)) is not True:
                state["ann_" + str(at)] = True
                return self._announce(state, budget, tl)
        return ""
