"""Daily Checkin Game - New API authoritative implementation."""

from .base import (
    GamePlugin,
    GameContext,
    GameResponse,
    SessionManager,
    BudgetChecker,
    EscrowEngine,
)
from .chatops_authority import call_chatops_authority


class CheckinGame(GamePlugin):
    name = "checkin"
    display_name = "签到"
    description = "群聊签到领取额度（New API 权威发放）"
    tier = "daily"
    triggers = ["签到", "打卡", "给我擦皮鞋", "checkin", "signin"]
    default_config = {
        "enabled": True,
        "max_per_user_day": 1,
        "budget_pool": "activity",
        "channel_scope": "all",
        "require_verify": True,
    }

    def can_play(self, ctx, budget):
        # 签到发放、重复签到、群级额度与资格均由 New API 权威接口判定。
        # 这里不使用本地内存 _cnt 限制，避免 adapter 重启/多入口/群级配置与后端台账不一致。
        if not self.config.get("enabled", True):
            return False, "disabled"
        return True, ""

    def handle(self, ctx, sm, budget, escrow):
        try:
            result = call_chatops_authority(ctx, "checkin")
        except Exception as e:
            return GameResponse.quick(
                f"@{ctx.username} 签到暂时失败：New API 权威接口不可用（{e}）"
            )
        if bool(result.get("success")):
            self.record_play(ctx.user_id)
        reply = result.get("reply") or (
            "签到成功" if result.get("success") else "签到失败"
        )
        return GameResponse(reply=reply, actions=[], event="checkin_authority")
