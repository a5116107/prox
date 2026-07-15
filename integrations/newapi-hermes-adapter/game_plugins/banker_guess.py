"""坐庄猜数 — 庄家设秘密数(1-100)，群友猜，最接近者赢"""

import random, time
from .base import (
    GamePlugin,
    GameContext,
    GameResponse,
    SessionManager,
    BudgetChecker,
    EscrowEngine,
)


class BankerGuessGame(GamePlugin):
    name = "banker_guess"
    display_name = "坐庄猜数"
    description = "庄家设数，群友竞猜1-100最接近者赢"
    tier = "pvp"
    triggers = ["坐庄", "开庄", "banker"]
    group_required = True
    default_config = {
        "enabled": True,
        "min_entry_usd": 0.05,
        "max_entry_usd": 1.00,
        "system_fee_pct": 0.15,
        "banker_bonus_pct": 0.05,
        "timeout_seconds": 180,
        "max_per_user_day": 20,
        "cooldown_seconds": 5,
        "budget_pool": "game",
    }

    def handle(self, ctx, sm, budget, escrow):
        t = ctx.text.strip()
        tl = t.lower()
        gid = ctx.group_id
        uid = ctx.user_id
        un = ctx.username

        # Start as banker: 坐庄 金额
        if any(tl.startswith(tr) for tr in ["坐庄", "开庄", "banker"]):
            sess = sm.get_active(gid)
            if sess and sess.game_name == "banker_guess":
                return GameResponse.quick(f"@{un} 本群已有进行中的猜数局，等结束后再开")

            parts = t.split()
            entry_usd = None
            for p in parts[1:]:
                try:
                    v = float(p.replace("$", ""))
                    if v > 0:
                        entry_usd = v
                except:
                    pass

            min_e = self.config["min_entry_usd"]
            max_e = self.config["max_entry_usd"]

            if entry_usd is None:
                return GameResponse.quick(
                    f"@{un} 🎰 **坐庄猜数**\n\n"
                    f"格式: `坐庄 入场费`\n"
                    f"例: `坐庄 0.10`\n"
                    f"入场费: ${min_e:.2f} ~ ${max_e:.2f}\n\n"
                    f"规则:\n"
                    f"1. 庄家自动设一个秘密数(1-100)\n"
                    f"2. 群友发 `猜 数字` 参与\n"
                    f"3. 庄家发 `开奖` 揭晓\n"
                    f"4. 最接近的人赢85%奖池，庄家拿5%"
                )

            entry_usd = max(min_e, min(max_e, entry_usd))
            entry_q = budget.usd_to_quota(entry_usd)
            secret = random.randint(1, 100)

            sm.start(
                gid,
                "banker_guess",
                self.config["timeout_seconds"],
                {
                    "type": "banker_guess",
                    "banker_id": uid,
                    "banker_napi": int(ctx.new_api_user_id or 0),
                    "banker_name": un,
                    "entry_q": entry_q,
                    "entry_usd": entry_usd,
                    "secret": secret,
                    "bets": {},
                    "started_at": time.time(),
                },
            )
            self.record_play(uid)
            return GameResponse(
                reply=(
                    f"@{un} 🎰 **开庄成功！**\n\n"
                    f"🔢 秘密数已设定 (1-100)\n"
                    f"💰 入场费: ${entry_usd:.2f}/次\n"
                    f"⏰ {self.config['timeout_seconds']}秒后自动开奖\n\n"
                    f"群友发送 `猜 数字` 参与，可多次竞猜\n"
                    f"庄家发 `开奖` 提前揭晓"
                ),
                event="banker_start",
            )

        # Guess: 猜 数字
        if tl.startswith("猜 ") or tl.startswith("猜数 "):
            sess = sm.get_active(gid)
            if not sess or sess.game_name != "banker_guess":
                return GameResponse.quick(f"@{un} 当前没有猜数局，发 `坐庄 金额` 开局")
            if uid == sess.state["banker_id"]:
                return GameResponse.quick(f"@{un} 庄家不能参与竞猜")

            parts = t.split()
            try:
                gnum = int(parts[1])
                if gnum < 1 or gnum > 100:
                    return GameResponse.quick(f"@{un} 请猜 1-100 的整数")
            except:
                return GameResponse.quick(f"@{un} 格式: `猜 42`")

            entry_q = sess.state["entry_q"]
            ok, why = budget.check_budget(self.config["budget_pool"], entry_q)
            if not ok:
                return GameResponse.quick(f"@{un} 游戏预算不足")

            budget.deduct(uid, entry_q, self.config["budget_pool"])
            if uid not in sess.state["bets"]:
                sess.state["bets"][uid] = {
                    "name": un,
                    "napi": int(ctx.new_api_user_id or 0),
                    "guesses": [],
                }
            else:
                sess.state["bets"][uid]["name"] = un
                sess.state["bets"][uid]["napi"] = int(
                    ctx.new_api_user_id or sess.state["bets"][uid].get("napi") or 0
                )
            sess.state["bets"][uid]["guesses"].append(gnum)

            n_players = len(sess.state["bets"])
            n_guesses = sum(len(d["guesses"]) for d in sess.state["bets"].values())
            self.record_play(uid)

            return GameResponse(
                reply=(
                    f"@{un} ✅ 竞猜成功！你猜的是 {gnum}\n"
                    f"💰 扣除入场费 ${sess.state['entry_usd']:.2f}\n"
                    f"👥 {n_players}人参与 / 共{n_guesses}次竞猜"
                ),
                actions=[
                    {
                        "type": "reward.grant.small",
                        "target_type": "user",
                        "user_id": ctx.new_api_user_id,
                        "quota_amount": -entry_q,
                        "budget_pool": self.config["budget_pool"],
                        "reason": "banker_guess_entry",
                    }
                ],
                event="banker_bet",
            )

        # Reveal: 开奖
        if tl in ("开奖", "揭晓", "reveal"):
            sess = sm.get_active(gid)
            if not sess or sess.game_name != "banker_guess":
                return GameResponse.quick(f"@{un} 当前没有猜数局")
            if uid != sess.state["banker_id"]:
                return GameResponse.quick(f"@{un} 只有庄家可以开奖")
            return self._reveal(sess, ctx, budget, sm)

        return None

    def _reveal(self, sess, ctx, budget, sm):
        secret = sess.state["secret"]
        bets = sess.state["bets"]
        gid = ctx.group_id

        if not bets:
            sm.end(gid)
            return GameResponse.quick(
                f"🎰 **开奖！** 秘密数: {secret}\n\n无人参与，本局作废"
            )

        total_guesses = sum(len(d["guesses"]) for d in bets.values())
        total_q = sess.state["entry_q"] * total_guesses
        fee = int(total_q * self.config["system_fee_pct"])
        banker_bonus = int(total_q * self.config["banker_bonus_pct"])
        prize = total_q - fee - banker_bonus

        budget.record_income(fee, self.config["budget_pool"])

        # Find winner (closest guess)
        best_diff = 999
        winner_id = None
        winner_name = ""
        for uid, d in bets.items():
            for g in d["guesses"]:
                diff = abs(g - secret)
                if diff < best_diff:
                    best_diff = diff
                    winner_id = uid
                    winner_name = d["name"]

        sm.end(gid)

        lines = [f"🎰 **开奖！** 秘密数: **{secret}**", ""]

        # Sort by closest
        sorted_bets = sorted(
            bets.items(), key=lambda x: min(abs(g - secret) for g in x[1]["guesses"])
        )
        for uid, d in sorted_bets:
            gs = ", ".join(str(g) for g in d["guesses"])
            diff = min(abs(g - secret) for g in d["guesses"])
            crown = " 🏆" if uid == winner_id else ""
            lines.append(f"  @{d['name']}: {gs} (差{diff}){crown}")

        lines.append("")
        lines.append(f"🏆 赢家: @{winner_name} +${budget.quota_to_usd(prize):.2f}")
        lines.append(
            f"🎩 庄家: @{sess.state['banker_name']} +${budget.quota_to_usd(banker_bonus):.2f}"
        )
        lines.append(f"💸 手续费: ${budget.quota_to_usd(fee):.2f}")

        winner_napi = int((bets.get(winner_id) or {}).get("napi") or 0)
        banker_napi = int(sess.state.get("banker_napi") or 0)
        actions = [
            {
                "type": "reward.grant.small",
                "target_type": "user",
                "user_id": winner_napi,
                "quota_amount": prize,
                "budget_pool": self.config["budget_pool"],
                "reason": "banker_win",
            },
            {
                "type": "reward.grant.small",
                "target_type": "user",
                "user_id": banker_napi,
                "quota_amount": banker_bonus,
                "budget_pool": self.config["budget_pool"],
                "reason": "banker_host",
            },
        ]
        return GameResponse(
            reply="\n".join(lines), actions=actions, event="banker_reveal"
        )
