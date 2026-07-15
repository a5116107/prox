import json
import os
import re
from chatops_client import (
    chatops_request,
    chatops_secret as _shared_chatops_secret,
    normalize_source,
)

from .base import GamePlugin, GameContext, GameResponse, format_card


class ProfileGame(GamePlugin):
    name = "profile"
    display_name = "我的资产"
    description = "查看个人余额和统计"
    tier = "utility"
    triggers = ["我的", "资产", "积分", "profile", "me", "余额"]
    default_config = {
        "enabled": True,
        "max_per_user_day": 99,
        "cooldown_seconds": 5,
        "budget_pool": "game",
    }

    def _chatops_secret(self):
        return _shared_chatops_secret()

    def _newapi_base(self):
        return (
            os.environ.get("NEWAPI_INTERNAL_BASE_URL")
            or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
            or "http://127.0.0.1:3000"
        ).rstrip("/")

    def _unwrap_action_result(self, out):
        """New API returns common.ApiSuccess(data=AgentAction).
        The live result is stored in data.result_json after force_execute.
        Keep this parser tolerant so older backends still work.
        """
        if not isinstance(out, dict) or not out.get("success"):
            return {}
        data = out.get("data") or {}
        if not isinstance(data, dict):
            return {}
        result = data.get("result") or data.get("result_json") or data.get("ResultJson")
        if isinstance(result, str) and result.strip():
            try:
                parsed = json.loads(result)
                if isinstance(parsed, dict):
                    parsed.setdefault("_action_status", data.get("status"))
                    return parsed
            except Exception:
                return {"summary": result, "_action_status": data.get("status")}
        if isinstance(result, dict):
            result.setdefault("_action_status", data.get("status"))
            return result
        return data

    def _try_profile_data(self, ctx):
        try:
            secret = self._chatops_secret()
            if not secret:
                return {}
            source = normalize_source(
                getattr(ctx, "platform", "")
                or os.environ.get("BOT_PLATFORM", "qq")
                or "qq"
            )
            uid = int(getattr(ctx, "new_api_user_id", 0) or 0)
            if uid <= 0:
                return {}
            payload = json.dumps(
                {
                    "source": source,
                    "room_id": str(getattr(ctx, "group_id", "") or ""),
                    "user_external_id": str(getattr(ctx, "user_id", "") or ""),
                    "username": str(getattr(ctx, "username", "") or ""),
                    "action": {
                        "type": "user.quota.read",
                        "new_api_user_id": uid,
                        "target_new_api_user_id": uid,
                        "user_id": uid,
                    },
                },
                ensure_ascii=False,
            ).encode("utf-8")
            out = chatops_request(
                "/api/agent/chatops/action",
                json.loads(payload.decode("utf-8")),
                source=source,
                timeout=5,
            )
            return self._unwrap_action_result(out)
        except Exception as e:
            return {"_profile_read_error": str(e)}

    def _extract_quota(self, data, fallback):
        for key in ("quota_balance", "balance_quota", "quota", "live_quota"):
            if isinstance(data, dict) and key in data and data.get(key) is not None:
                try:
                    return int(float(data.get(key))), "实时读取"
                except Exception:
                    pass
        summary = str((data or {}).get("summary") or "")
        m = re.search(r"原始额度\s*=\s*(-?\d+)", summary)
        if m:
            try:
                return int(m.group(1)), "实时读取"
            except Exception:
                pass
        m = re.search(r"当前余额[：:]\s*\$?([0-9]+(?:\.[0-9]+)?)", summary)
        if m:
            try:
                return int(float(m.group(1)) * 500000), "实时读取"
            except Exception:
                pass
        try:
            return int(fallback or 0), "绑定缓存"
        except Exception:
            return 0, "绑定缓存"

    def handle(self, ctx, sm, budget, escrow):
        if not ctx.new_api_user_id:
            return GameResponse.quick(f"@{ctx.username} 请先「验牌」绑定账号后查看资产")

        extra = self._try_profile_data(ctx)
        balance_q, source = self._extract_quota(extra, ctx.quota_balance or 0)
        balance_usd = budget.quota_to_usd(balance_q)

        fields = [
            ("玩家", f"@{ctx.username}"),
            ("余额", f"${balance_usd:.2f}"),
            ("数据", source),
        ]

        if isinstance(extra, dict):
            if extra.get("checkin_streak"):
                fields.append(("连签", f"{extra['checkin_streak']} 天"))
            if extra.get("total_checkins"):
                fields.append(("累计签到", f"{extra['total_checkins']} 次"))
            if extra.get("total_earned_usd") is not None:
                try:
                    fields.append(
                        ("累计收益", f"${float(extra['total_earned_usd']):.2f}")
                    )
                except Exception:
                    pass
            if extra.get("games_played") is not None:
                fields.append(("游戏次数", str(extra["games_played"])))
            if extra.get("invite_count") is not None:
                fields.append(("邀请人数", str(extra["invite_count"])))
            if extra.get("_profile_read_error"):
                fields.append(("提示", "实时余额读取失败，已显示缓存值"))

        card = format_card(
            "我的资产",
            fields,
            footer="发送「菜单」查看可用游戏",
            emoji="💎",
        )

        self.record_play(ctx.user_id)
        return GameResponse.quick(f"@{ctx.username}\n{card}")
