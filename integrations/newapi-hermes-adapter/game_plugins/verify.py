"""Verify Game - New API authoritative binding/API key/quota validation."""

from .base import (
    GamePlugin,
    GameContext,
    GameResponse,
    SessionManager,
    BudgetChecker,
    EscrowEngine,
)
from .chatops_authority import call_chatops_authority


class VerifyGame(GamePlugin):
    name = "verify"
    display_name = "验牌"
    description = "New API 权威三项验牌：绑定 / API Key / 额度"
    tier = "daily"
    triggers = ["验牌", "我要验牌", "验证", "verify"]
    default_config = {
        "enabled": True,
        "max_per_user_day": 99,
        "cost_quota": 0,
        "budget_pool": "game",
    }

    def handle(self, ctx, sm, budget, escrow):
        self.record_play(ctx.user_id)
        try:
            result = call_chatops_authority(ctx, "verify")
        except Exception as e:
            return GameResponse(
                reply=f"@{ctx.username} 验牌暂时失败：New API 权威接口不可用（{e}）",
                event="verify_fail",
            )
        reply = result.get("reply") or (
            "验牌通过" if result.get("passed") else "验牌未通过"
        )
        return GameResponse(
            reply=reply,
            actions=[],
            event="verify_pass" if result.get("passed") else "verify_fail",
        )
