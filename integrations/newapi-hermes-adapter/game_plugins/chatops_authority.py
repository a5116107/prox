"""New API chatops authority helpers for adapter game plugins.

The adapter must not be the financial/identity source of truth.  Checkin and
verify call New API authoritative endpoints and only render the returned reply.
"""

import json
import os
from chatops_client import (
    chatops_request,
    chatops_secret as _shared_chatops_secret,
    normalize_source,
)


def _ctx_value(ctx, name, default=""):
    value = getattr(ctx, name, default)
    if value is None:
        return default
    return value


def chatops_source(ctx):
    platform = str(_ctx_value(ctx, "platform", "") or "").strip().lower()
    if platform in ("community", "dc", "hhhl"):
        return "community"
    return normalize_source(platform)


def newapi_base_url():
    return (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL")
        or os.environ.get("NEWAPI_BASE_URL")
        or "http://127.0.0.1:3000"
    ).rstrip("/")


def chatops_secret():
    return _shared_chatops_secret()


def call_chatops_authority(ctx, op, text=None, timeout=8):
    """Call /api/agent/chatops/checkin|verify and return New API data dict."""
    source = chatops_source(ctx)
    base = newapi_base_url()
    secret = chatops_secret()
    if not secret:
        raise RuntimeError("missing CHATOPS_WEBHOOK_SECRET/NEWAPI_CHATOPS_SECRET")
    endpoint = (
        "/api/agent/chatops/checkin" if op == "checkin" else "/api/agent/chatops/verify"
    )
    payload = {
        "source": source,
        "room_id": str(_ctx_value(ctx, "group_id", "") or ""),
        "message_id": str(_ctx_value(ctx, "message_id", "") or ""),
        "user_external_id": str(_ctx_value(ctx, "user_id", "") or ""),
        "username": str(_ctx_value(ctx, "username", "") or ""),
        "text": text if text is not None else str(_ctx_value(ctx, "text", "") or ""),
        "new_api_user_id": int(_ctx_value(ctx, "new_api_user_id", 0) or 0),
        "user_bound": bool(_ctx_value(ctx, "is_bound", False)),
        "raw": {
            "adapter": "newapi-hermes-adapter",
            "site_id": str(_ctx_value(ctx, "site_id", "") or ""),
            "platform": str(_ctx_value(ctx, "platform", "") or ""),
        },
    }
    out = chatops_request(endpoint, payload, source=source, timeout=timeout)
    if isinstance(out, dict) and out.get("success") is False:
        raise RuntimeError(
            out.get("message") or out.get("error") or "new api chatops failed"
        )
    result = out.get("data") if isinstance(out, dict) else out
    if not isinstance(result, dict):
        raise RuntimeError("invalid new api chatops response")
    return result
