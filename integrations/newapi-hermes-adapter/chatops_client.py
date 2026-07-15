"""Shared New API ChatOps internal client.

Live New API deployments authorize internal ChatOps calls differently by
source: QQ/community/admin accept the `secret` query parameter, while TG also
requires `X-Telegram-Bot-Api-Secret-Token`.  Keep every adapter-side caller on
this helper so binding, checkin, verify, invite, profile and config sync do not
drift back to Bearer-only auth.
"""

import json
import os
import urllib.parse
import urllib.request


def normalize_source(source=""):
    s = str(source or "").strip().lower()
    if s == "telegram":
        return "tg"
    return s or "qq"


def chatops_secret():
    return (
        os.environ.get("CHATOPS_WEBHOOK_SECRET")
        or os.environ.get("NEWAPI_CHATOPS_SECRET")
        or os.environ.get("QQ_BRIDGE_SECRET")
        or os.environ.get("ADAPTER_KEY")
        or ""
    ).strip()


def newapi_base_url():
    return (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL")
        or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
        or os.environ.get("NEWAPI_BASE_URL")
        or "http://127.0.0.1:3000"
    ).rstrip("/")


def chatops_url(path, source="qq", base=None, secret=None, extra_query=None):
    src = normalize_source(source)
    sec = secret if secret is not None else chatops_secret()
    query = dict(extra_query or {})
    query["secret"] = sec
    if src:
        query["source"] = src
    return (
        (base or newapi_base_url()).rstrip("/")
        + path
        + "?"
        + urllib.parse.urlencode(query)
    )


def chatops_headers(source="qq", secret=None, content_type="application/json"):
    src = normalize_source(source)
    sec = secret if secret is not None else chatops_secret()
    headers = {}
    if content_type:
        headers["Content-Type"] = content_type
    if src == "tg" and sec:
        headers["X-Telegram-Bot-Api-Secret-Token"] = sec
    return headers


def chatops_request(
    path, payload=None, source="qq", timeout=8, method="POST", extra_query=None
):
    sec = chatops_secret()
    if not sec:
        raise RuntimeError("missing CHATOPS_WEBHOOK_SECRET/NEWAPI_CHATOPS_SECRET")
    data = None
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        chatops_url(path, source=source, secret=sec, extra_query=extra_query),
        data=data,
        headers=chatops_headers(source=source, secret=sec),
        method=method,
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        raw = resp.read().decode("utf-8") or "{}"
    return json.loads(raw)
