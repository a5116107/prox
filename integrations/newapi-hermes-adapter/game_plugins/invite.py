"""Invite Reward — New API authoritative invite chain.

Adapter/TG/QQ only report link/join/verify events. New API owns:
invite_links -> invite_edges -> invite_events -> invite_reward_claims -> ops_fund_ledgers.
"""

import json
import os
from chatops_client import (
    chatops_request,
    chatops_secret as _shared_chatops_secret,
    normalize_source,
)

from .base import GamePlugin, GameResponse


class InviteGame(GamePlugin):
    name = "invite"
    display_name = "邀请奖励"
    description = "邀请新用户进群并验牌，奖励由 New API 权威落表和发放"
    tier = "binding"
    triggers = ["邀请", "我的邀请", "invite", "myinvite", "邀请链接"]
    default_config = {
        "enabled": True,
        "inviter_reward_quota": 1500000,
        "invitee_reward_quota": 750000,
        "max_per_user_day": 10,
        "cooldown_seconds": 0,
        "budget_pool": "community",
        "pending_expire_hours": 72,
    }

    def _source(self, ctx=None):
        s = (
            str(
                getattr(ctx, "platform", "")
                or os.environ.get("CHATOPS_SOURCE", "")
                or "qq"
            )
            .lower()
            .strip()
        )
        return normalize_source(s)

    def _site_url(self, ctx=None):
        return os.environ.get(
            "PUBLIC_BASE_URL", f"https://ai.{getattr(ctx, 'site_id', 'newapi')}.us.ci"
        ).rstrip("/")

    def _secret(self):
        return _shared_chatops_secret()

    def _base(self):
        return (
            os.environ.get("NEWAPI_INTERNAL_BASE_URL") or "http://127.0.0.1:3000"
        ).rstrip("/")

    def _campaign(self, source, room_id, site_id=""):
        room = (
            "".join(ch for ch in str(room_id or "private") if ch.isalnum())[:48]
            or "private"
        )
        site = (
            "".join(
                ch
                for ch in str(site_id or os.environ.get("SITE_ID") or "site")
                if ch.isalnum()
            )[:32]
            or "site"
        )
        return f"chatops-{site}-{source}-{room}"

    def _api(self, action, ctx=None, **extra):
        secret = self._secret()
        if not secret:
            raise RuntimeError("chatops secret not configured")
        source = extra.pop("source", None) or self._source(ctx)
        room_id = str(extra.pop("room_id", None) or getattr(ctx, "group_id", "") or "")
        payload = {
            "action": action,
            "source": source,
            "room_id": room_id,
            "user_external_id": str(
                extra.pop("user_external_id", None) or getattr(ctx, "user_id", "") or ""
            ),
            "username": str(
                extra.pop("username", None) or getattr(ctx, "username", "") or ""
            ),
            "new_api_user_id": int(
                extra.pop("new_api_user_id", None)
                or getattr(ctx, "new_api_user_id", 0)
                or 0
            ),
            "campaign_code": extra.pop("campaign_code", None)
            or self._campaign(source, room_id, getattr(ctx, "site_id", "")),
            "inviter_reward_quota": int(
                self.config.get(
                    "inviter_reward_quota", self.config.get("reward_quota", 1500000)
                )
                or 0
            ),
            "invitee_reward_quota": int(
                self.config.get("invitee_reward_quota", 750000) or 0
            ),
            "budget_pool": self.config.get("budget_pool", "community"),
        }
        payload.update({k: v for k, v in extra.items() if v is not None})
        out = chatops_request(
            "/api/agent/chatops/invite", payload, source=source, timeout=6
        )
        if not out.get("success"):
            raise RuntimeError(out.get("message") or "invite api failed")
        return out.get("data") or {}

    def handle(self, ctx, sm, budget, escrow):
        t = ctx.text.strip().lower()
        if any(kw in t for kw in ["我的邀请", "my", "邀请统计", "myinvite"]):
            return self._show_stats(ctx, budget)
        if any(kw in t for kw in ["邀请链接", "invite link", "链接", "邀请", "invite"]):
            return self._show_link(ctx, budget)
        return self._show_link(ctx, budget)

    def _show_link(self, ctx, budget):
        if not ctx.new_api_user_id:
            return GameResponse.quick(
                f"@{ctx.username} 请先「验牌」绑定账号，才能生成邀请链接"
            )
        aff_link = f"{self._site_url(ctx)}/sign-up?aff={ctx.new_api_user_id}"
        try:
            data = self._api("link", ctx, invite_url=aff_link)
            aff_link = data.get("invite_url") or aff_link
        except Exception as e:
            return GameResponse.quick(f"@{ctx.username} 邀请链接生成失败：{e}")
        inviter_r = budget.quota_to_usd(int(data.get("inviter_reward_quota") or 0))
        invitee_r = budget.quota_to_usd(int(data.get("invitee_reward_quota") or 0))
        return GameResponse.quick(
            f"@{ctx.username} 🔗 你的邀请链接:\n\n{aff_link}\n\n"
            f"好友通过链接注册或由你拉入群后，完成「验牌」即自动结算。\n"
            f"邀请人 +${inviter_r:.2f} / 被邀请人 +${invitee_r:.2f}"
        )

    def _show_stats(self, ctx, budget):
        if not getattr(ctx, "new_api_user_id", 0):
            return GameResponse.quick(
                f"@{ctx.username} 暂时查不到邀请统计：你的群聊身份还没有绑定到站点账号。\n\n"
                f"请先发送「验牌」确认账号；如提示未绑定，请在站点登录后获取绑定码，再回群发送「绑定 绑定码」。"
            )
        try:
            data = self._api("stats", ctx)
            st = data.get("stats") or {}
            earned = budget.quota_to_usd(int(st.get("earned_quota") or 0))
            return GameResponse.quick(
                f"@{ctx.username} 📊 邀请统计\n\n"
                f"🔗 链接数：{st.get('links', 0)}\n"
                f"👥 进群记录：{st.get('joins', 0)}\n"
                f"✅ 验牌闭环：{st.get('verified', 0)}\n"
                f"💰 已发邀请奖励：{st.get('paid', 0)} 笔 / ${earned:.2f}\n"
            )
        except Exception as e:
            return GameResponse.quick(f"@{ctx.username} 邀请统计查询失败：{e}")

    def handle_notice_increase(self, gid, new_uid, operator_id=None):
        if not operator_id or str(operator_id) == str(new_uid):
            return None
        try:
            self._api(
                "join",
                None,
                source=os.environ.get("CHATOPS_SOURCE")
                or os.environ.get("BOT_PLATFORM")
                or "qq",
                room_id=str(gid),
                user_external_id=str(new_uid),
                invitee_external_id=str(new_uid),
                inviter_external_id=str(operator_id),
                operator_external_id=str(operator_id),
                metadata={"event": "group_increase"},
            )
        except Exception as e:
            print(
                f"[Invite] join report failed gid={gid} new={new_uid} op={operator_id}: {e}",
                flush=True,
            )
        return None

    def check_verify_reward(
        self, uid, username="", new_api_user_id=0, group_id="", platform="qq"
    ):
        try:
            return self._api(
                "verify_claim",
                None,
                source=platform or os.environ.get("CHATOPS_SOURCE", "qq"),
                room_id=str(group_id or ""),
                user_external_id=str(uid),
                invitee_external_id=str(uid),
                username=username,
                new_api_user_id=int(new_api_user_id or 0),
                invitee_user_id=int(new_api_user_id or 0),
                metadata={"event": "verify_pass"},
            )
        except Exception as e:
            print(
                f"[Invite] verify claim failed uid={uid} napi={new_api_user_id}: {e}",
                flush=True,
            )
            return None
