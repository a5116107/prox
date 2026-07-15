"""Lottery (双色球) — Pick 6 red balls (1-33) + 1 blue ball (1-16). Hourly draw with prize tiers."""

import random, time, json, os, threading
from .base import GamePlugin, GameContext, GameResponse


class LotteryGame(GamePlugin):
    name = "lottery"
    display_name = "彩票双色球"
    description = "双色球：选6红(1-33)+1蓝(1-16)，每小时开奖，分级奖金"
    tier = "interactive"
    triggers = ["彩票", "买彩票", "选号", "机选", "开奖", "lottery", "pick"]
    group_required = True
    default_config = {
        "enabled": True,
        "ticket_cost_usd": 0.05,
        "max_tickets_per_user": 10,
        "draw_interval_sec": 3600,
        "jackpot_start_usd": 1.00,
        "commission_rate": 0.15,
        "max_per_user_day": 50,
        "budget_pool": "activity",
        "prize_split": {
            "6+1": 0.60,
            "6+0": 0.20,
            "5+1": 0.12,
            "5+0": 0.05,
            "4+1": 0.03,
        },
    }

    def __init__(self):
        super().__init__()
        self._tickets = {}
        self._jackpot = {}
        self._last_draw = {}
        self._draw_threads = {}
        self._winner_history = {}
        self._lock = threading.Lock()
        self._cost_quota = int(self.default_config["ticket_cost_usd"] * 500000)

    def _config_quota(self, key):
        v = self.config.get(key + "_quota", None)
        if v is None:
            v_usd = self.config.get(
                key + "_usd", self.default_config.get(key + "_usd", 0)
            )
            v = int(v_usd * 500000)
        return v

    def _config_usd(self, key):
        return self.config.get(key + "_usd", self.default_config.get(key + "_usd", 0))

    def handle(self, ctx, sm, budget, escrow):
        t = ctx.text.strip().lower()
        gid = ctx.group_id or ctx.user_id
        uid = ctx.user_id
        un = ctx.username

        self._cost_quota = self._config_quota("ticket_cost")
        cost_usd = self._config_usd("ticket_cost")

        if "开奖" in t or "draw" in t:
            return self._manual_draw(gid, uid, un)

        if t in ["彩票", "lottery", "状态"] or (t.startswith("彩票") and len(t) <= 4):
            return self._show_status(gid, un)

        if t.startswith("选号"):
            nums = self._parse_numbers(t)
            if not nums:
                return GameResponse.quick(
                    f"@{un} 🎰 选号格式: 6个红球(1-33) + 1个蓝球(1-16)\n"
                    f"示例: 选号 01 05 12 18 25 30 07\n"
                    f"或: 机选 3 (随机3注)"
                )
            return self._buy_ticket(gid, uid, un, nums, cost_usd, budget, ctx)

        if "机选" in t:
            try:
                count = 1
                parts = t.split()
                for p in parts:
                    if p.isdigit():
                        count = min(int(p), self.config["max_tickets_per_user"])
                        break
            except:
                count = 1
            msgs = []
            all_actions = []
            for _ in range(count):
                nums = self._random_pick()
                r = self._buy_ticket(gid, uid, un, nums, cost_usd, budget, ctx)
                msgs.append(r.reply)
                if r.event == "lottery_buy":
                    all_actions.extend(r.actions or [])
                else:
                    break
            return GameResponse(
                reply="\n".join(msgs),
                actions=all_actions,
                event="lottery_buy" if all_actions else None,
            )

        if "买彩票" in t or "买" in t:
            try:
                count = 1
                parts = t.split()
                for p in parts:
                    if p.isdigit():
                        count = min(int(p), self.config["max_tickets_per_user"])
                        break
            except:
                count = 1
            msgs = []
            all_actions = []
            for _ in range(count):
                nums = self._random_pick()
                r = self._buy_ticket(gid, uid, un, nums, cost_usd, budget, ctx)
                msgs.append(r.reply)
                if r.event == "lottery_buy":
                    all_actions.extend(r.actions or [])
                else:
                    break
            return GameResponse(
                reply="\n".join(msgs),
                actions=all_actions,
                event="lottery_buy" if all_actions else None,
            )

        return None

    def _parse_numbers(self, t):
        import re

        nums = re.findall(r"\d+", t.replace("选号", ""))
        if len(nums) < 7:
            return None
        try:
            reds = sorted(set(int(n) for n in nums[:6]))
            blue = int(nums[6])
            if len(reds) != 6:
                return None
            if min(reds) < 1 or max(reds) > 33:
                return None
            if blue < 1 or blue > 16:
                return None
            return {"reds": reds, "blue": blue}
        except:
            return None

    def _random_pick(self):
        return {
            "reds": sorted(random.sample(range(1, 34), 6)),
            "blue": random.randint(1, 16),
        }

    def _buy_ticket(self, gid, uid, un, nums, cost_usd, budget, ctx):
        current_tickets = [t for t in self._tickets.get(gid, []) if t["uid"] == uid]
        if len(current_tickets) >= self.config["max_tickets_per_user"]:
            return GameResponse.quick(
                f"@{un} ⚠️ 本轮已购买 {len(current_tickets)} 注，达到上限"
            )

        cost_quota = self._cost_quota
        ok, err = budget.check_budget(self.config["budget_pool"], cost_quota)
        if not ok:
            return GameResponse.quick(f"@{un} ❌ 预算不足: {err}")
        ok, err = budget.check_user_limit(uid, cost_quota, self.config["budget_pool"])
        if not ok:
            return GameResponse.quick(f"@{un} ❌ {err}")

        budget.deduct(uid, cost_quota, self.config["budget_pool"])

        commission_rate = self.config.get("commission_rate", 0.15)
        commission_usd = cost_usd * commission_rate
        commission_quota = int(commission_usd * 500000)
        budget.record_income(commission_quota, self.config["budget_pool"])

        jackpot_add = cost_usd - commission_usd
        start_jp = self._config_usd("jackpot_start")
        self._jackpot[gid] = self._jackpot.get(gid, start_jp)
        if self._jackpot.get(gid, 0) < start_jp * 0.5:
            self._jackpot[gid] = max(self._jackpot.get(gid, start_jp), start_jp * 0.5)
        self._jackpot[gid] = self._jackpot.get(gid, 0) + jackpot_add

        ticket_id = len(self._tickets.get(gid, [])) + 1
        ticket = {
            "id": ticket_id,
            "uid": uid,
            "napi": int(ctx.new_api_user_id or 0),
            "username": un,
            "reds": nums["reds"],
            "blue": nums["blue"],
        }
        self._tickets.setdefault(gid, []).append(ticket)

        reds_str = " ".join(f"{n:02d}" for n in nums["reds"])
        blue_str = f"{nums['blue']:02d}"
        all_tickets = self._tickets.get(gid, [])
        my_count = len([t for t in all_tickets if t["uid"] == uid])

        msg = (
            f"@{un} 🎫 第 {ticket_id} 注购买成功！\n"
            f"📌 红球: {reds_str} | 🔵 蓝球: {blue_str}\n"
            f"💸 花费: ${cost_usd:.2f} (抽成 {int(commission_rate * 100)}%)"
            f" | 📊 本轮已购: {my_count} 注\n"
            f"🏆 当前奖池: ${self._jackpot.get(gid, 0):.2f}"
        )
        return GameResponse(
            reply=msg,
            actions=[
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": ctx.new_api_user_id,
                    "quota_amount": -cost_quota,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "lottery_ticket",
                }
            ],
            event="lottery_buy",
        )

    def _show_status(self, gid, un):
        jackpot = self._jackpot.get(gid, self._config_usd("jackpot_start"))
        start_jp = self._config_usd("jackpot_start")
        if jackpot < start_jp * 0.5:
            jackpot = start_jp
            self._jackpot[gid] = start_jp

        tickets = self._tickets.get(gid, [])
        total_tickets = len(tickets)
        unique_players = len(set(t["uid"] for t in tickets))

        elapsed = time.time() - self._last_draw.get(gid, time.time())
        interval = self.config["draw_interval_sec"]
        remaining = max(0, interval - elapsed)
        mins = int(remaining // 60)
        secs = int(remaining % 60)

        winners = self._winner_history.get(gid, [])[-3:]

        msg = (
            f"@{un} 🎰 双色球\n"
            f"━━━━━━━━━━━━━━\n"
            f"🏆 奖池: ${jackpot:.2f}\n"
            f"🎫 已售: {total_tickets} 注 ({unique_players} 人)\n"
            f"⏰ 开奖: {mins}分{secs}秒后\n"
            f"💵 单价: ${self._config_usd('ticket_cost'):.2f}/注"
            f" (抽成 {int(self.config.get('commission_rate', 0.15) * 100)}%)\n"
        )
        if winners:
            msg += "━━━━━━━━━━━━━━\n📜 近期中奖:\n"
            for w in winners:
                msg += f"  {w['username']} ${w['amount']:.2f} ({w['prize_tier']})\n"
        msg += "━━━━━━━━━━━━━━\n💡 选号 <6红+1蓝> | 机选 <数量> | 买彩票"

        return GameResponse.quick(msg)

    def _manual_draw(self, gid, uid, un):
        return self._do_draw(gid)

    def _do_draw(self, gid):
        tickets = self._tickets.get(gid, [])
        jackpot = self._jackpot.get(gid, self._config_usd("jackpot_start"))
        start_jp = self._config_usd("jackpot_start")
        if jackpot < start_jp * 0.5:
            jackpot = start_jp

        if not tickets:
            self._last_draw[gid] = time.time()
            new_jp = jackpot * 0.20
            self._jackpot[gid] = max(new_jp, start_jp * 0.5)
            self._tickets[gid] = []
            return GameResponse.quick(
                f"🎰 双色球开奖\n━━━━━━━━━━━━━━\n"
                f"📭 本轮无人购买，奖池 ${jackpot:.2f} → 下轮 ${self._jackpot[gid]:.2f}\n"
                f"⏰ 下一轮：{self.config['draw_interval_sec'] // 60} 分钟后"
            )

        win_reds = sorted(random.sample(range(1, 34), 6))
        win_blue = random.randint(1, 16)

        prize_split = self.config.get("prize_split", self.default_config["prize_split"])
        winners = {"6+1": [], "6+0": [], "5+1": [], "5+0": [], "4+1": [], "none": []}
        for ticket in tickets:
            matched_reds = len(set(ticket["reds"]) & set(win_reds))
            matched_blue = ticket["blue"] == win_blue
            if matched_reds == 6 and matched_blue:
                tier = "6+1"
            elif matched_reds == 6:
                tier = "6+0"
            elif matched_reds == 5 and matched_blue:
                tier = "5+1"
            elif matched_reds == 5:
                tier = "5+0"
            elif matched_reds == 4 and matched_blue:
                tier = "4+1"
            else:
                tier = "none"
            winners[tier].append(ticket)

        distributions = []
        total_paid = 0
        for tier, tlist in winners.items():
            if tier == "none" or not tlist:
                continue
            share = prize_split.get(tier, 0)
            pool = jackpot * share
            per_winner = pool / len(tlist) if tlist else 0
            for ticket in tlist:
                distributions.append(
                    {
                        "uid": int(ticket.get("napi") or 0),
                        "external_uid": ticket["uid"],
                        "username": ticket["username"],
                        "amount": per_winner,
                        "prize_tier": tier,
                        "reds": ticket["reds"],
                        "blue": ticket["blue"],
                    }
                )
                total_paid += per_winner

        remaining = jackpot - total_paid
        roll_forward = remaining * 0.20
        self._jackpot[gid] = max(roll_forward + start_jp * 0.5, start_jp * 0.5)

        self._winner_history.setdefault(gid, []).extend(distributions)
        if len(self._winner_history.get(gid, [])) > 20:
            self._winner_history[gid] = self._winner_history[gid][-20:]

        self._tickets[gid] = []
        self._last_draw[gid] = time.time()

        win_reds_str = " ".join(f"{n:02d}" for n in win_reds)
        msg = (
            f"🎰 双色球开奖结果\n"
            f"━━━━━━━━━━━━━━\n"
            f"📌 红球: {win_reds_str} | 🔵 蓝球: {win_blue:02d}\n"
            f"🏆 奖池: ${jackpot:.2f} | 售出: {len(tickets)} 注\n"
            f"━━━━━━━━━━━━━━\n"
        )

        if distributions:
            msg += "🎊 中奖名单:\n"
            for d in sorted(distributions, key=lambda x: -x["amount"]):
                reds_str = " ".join(f"{n:02d}" for n in d["reds"])
                msg += f"  {d['username']} ${d['amount']:.2f} [{d['prize_tier']}] {reds_str}+{d['blue']:02d}\n"
        else:
            msg += "📭 本轮无人中奖\n"

        msg += (
            f"━━━━━━━━━━━━━━\n"
            f"💰 下轮奖池: ${self._jackpot[gid]:.2f}\n"
            f"⏰ 下一轮：{self.config['draw_interval_sec'] // 60} 分钟后"
        )

        total_commission = (
            len(tickets)
            * self._config_usd("ticket_cost")
            * self.config.get("commission_rate", 0.15)
        )
        msg += f"\n💼 平台抽成: ${total_commission:.2f} ({int(self.config.get('commission_rate', 0.15) * 100)}%)"

        actions = []
        for d in distributions:
            quota_amount = int(float(d.get("amount") or 0) * 500000)
            if quota_amount > 0 and int(d.get("uid") or 0) > 0:
                actions.append(
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": int(d.get("uid") or 0),
                        "quota_amount": quota_amount,
                        "budget_pool": self.config["budget_pool"],
                        "reason": f"lottery_win_{d.get('prize_tier', 'prize')}",
                    }
                )
        return GameResponse(reply=msg, actions=actions, event="lottery_draw")

    def start_bg_tasks(self, budget):
        self._last_budget = budget
        self._schedule_next_draw("default", budget)

    def _schedule_next_draw(self, gid, budget):
        interval = self.config["draw_interval_sec"]
        timer = threading.Timer(interval, self._auto_draw, args=[gid, budget])
        timer.daemon = True
        self._draw_threads[gid] = timer
        timer.start()

    def _auto_draw(self, gid, budget):
        self._do_draw(gid)
        self._schedule_next_draw(gid, budget)
