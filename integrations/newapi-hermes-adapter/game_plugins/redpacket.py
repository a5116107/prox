"""Red Packet Rain (红包雨) - admin funded, timed red packet distribution."""

import random
import time

from .base import GamePlugin, GameResponse


class RedPacketGame(GamePlugin):
    name = "redpacket"
    display_name = "红包雨"
    description = "管理员发红包，群友限时抢"
    tier = "interactive"
    triggers = ["红包", "红包雨", "redpacket", "发红包", "抢红包"]
    group_required = True
    default_config = {
        "enabled": True,
        "min_packets": 5,
        "max_packets": 50,
        "min_total_usd": 0.50,
        "max_total_usd": 5.00,
        "grab_window_sec": 30,
        "max_per_user_day": 20,
        "cooldown_seconds": 0,
        "budget_pool": "activity",
    }

    def _finalize_expired(self, ctx, sm, state, budget):
        grabbed = len(state.get("grabbed", {}))
        total_packets = int(state.get("tot_packets") or 0)
        total_quota = int(state.get("tot_quota") or 0)
        disbursed = int(state.get("disbursed") or 0)
        remaining = max(total_quota - disbursed, 0)
        owner_uid = int(state.get("owner_new_api_user_id") or 0)
        owner_external = str(state.get("owner_user_id") or "")
        actions = []
        if remaining > 0 and owner_uid > 0:
            actions.append(
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": owner_uid,
                    "target_external_id": owner_external,
                    "quota_amount": remaining,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "redpacket_refund",
                    "source_type": "game_refund",
                }
            )
            tail = f"💸 未抢部分 ${budget.quota_to_usd(remaining):.2f} 已退回发起人"
        else:
            tail = "💸 未抢部分已结算"
        try:
            sm.end(ctx.group_id)
        except Exception:
            pass
        reply = f"🧧 红包雨已结束\n📦 已抢 {grabbed}/{total_packets}\n{tail}"
        return GameResponse(reply=reply, actions=actions, event="redpacket_refund")

    def handle(self, ctx, sm, budget, escrow):
        t = (ctx.text or "").strip().lower()
        uid = str(ctx.user_id)
        un = ctx.username

        # Active/expired sessions are inspected directly. get_active() hides
        # expired sessions, but refund settlement must see the original state.
        active = (
            sm.get_session(ctx.group_id)
            if hasattr(sm, "get_session")
            else sm.get_active(ctx.group_id)
        )
        if active and active.game_name == "redpacket":
            state = active.state or {}
            if active.is_expired() or t in ("__auto_close__", "__auto_expire__"):
                return self._finalize_expired(ctx, sm, state, budget)

            if uid in state.get("grabbed", {}):
                return GameResponse.quick(
                    f"@{un} 你已经抢过了~ 抢到 ${budget.quota_to_usd(state['grabbed'][uid]):.2f}"
                )
            if len(state.get("grabbed", {})) >= int(state.get("tot_packets") or 0):
                return GameResponse.quick(f"@{un} 红包已抢完！")

            remaining = int(state.get("tot_quota") or 0) - int(
                state.get("disbursed") or 0
            )
            remaining_packets = int(state.get("tot_packets") or 0) - len(
                state.get("grabbed", {})
            )
            if remaining_packets <= 0 or remaining <= 0:
                return GameResponse.quick(f"@{un} 红包已抢完！")

            if remaining_packets == 1:
                grab_q = remaining
            else:
                avg = remaining // remaining_packets
                low = max(1000, avg // 2)
                high = min(remaining - (remaining_packets - 1) * 1000, avg * 2)
                high = max(high, low)
                grab_q = random.randint(low, high)
            grab_q = max(1000, min(grab_q, remaining))

            state.setdefault("grabbed", {})[uid] = grab_q
            state["disbursed"] = int(state.get("disbursed") or 0) + grab_q
            self.record_play(uid)

            msg = f"@{un} 🧧 抢到 +${budget.quota_to_usd(grab_q):.2f}！"
            msg += f"\n  ({len(state['grabbed'])}/{state['tot_packets']} 已抢, 剩 {budget.quota_to_usd(remaining - grab_q):.2f} 未分配)"
            return GameResponse(
                reply=msg,
                actions=[
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ctx.new_api_user_id,
                        "target_external_id": uid,
                        "quota_amount": grab_q,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "redpacket_grab",
                    }
                ],
                event="redpacket_grab",
            )

        if not ctx.is_admin:
            return GameResponse.quick(
                f"@{un} 当前没有可抢的红包雨，等待管理员发送「红包雨 数量 金额」发起。"
            )

        parts = t.split()
        count = int(self.config["min_packets"])
        total_usd = 0.50
        for p in parts[1:]:
            try:
                v = int(p)
                if 1 <= v <= int(self.config["max_packets"]):
                    count = v
                    continue
            except Exception:
                pass
            try:
                v2 = float(p)
                if v2 > 0:
                    total_usd = v2
            except Exception:
                pass

        total_usd = min(
            float(self.config["max_total_usd"]),
            max(float(self.config["min_total_usd"]), total_usd or 0.50),
        )
        total_q = budget.usd_to_quota(total_usd)

        sm.start(
            ctx.group_id,
            "redpacket",
            int(self.config["grab_window_sec"]),
            {
                "tot_packets": count,
                "tot_quota": total_q,
                "grabbed": {},
                "disbursed": 0,
                "owner_user_id": str(ctx.user_id),
                "owner_username": un,
                "owner_new_api_user_id": int(ctx.new_api_user_id or 0),
                "platform": str(ctx.platform or "unknown"),
                "started_at": time.time(),
            },
        )
        budget.deduct(ctx.user_id, total_q, self.config["budget_pool"])

        msg = (
            f"@{un} 🧧 **红包雨来袭！**\n"
            f"📦 {count} 个红包 | 💰 总额 ${total_usd:.2f}\n"
            f"⏰ {self.config['grab_window_sec']}秒内有效！\n\n"
            f"💡 输入「红包」来抢！"
        )
        return GameResponse(
            reply=msg,
            actions=[
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": ctx.new_api_user_id,
                    "target_external_id": str(ctx.user_id),
                    "quota_amount": -total_q,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "redpacket_create",
                }
            ],
            event="redpacket_create",
        )
