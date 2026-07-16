from __future__ import annotations

import importlib.util
import hashlib
import hmac
import io
import json
import sys
import threading
import time
import urllib.error
from pathlib import Path

import pytest


SITE_ROOT = Path(__file__).resolve().parents[1]
ADAPTER_PATH = SITE_ROOT / "adapter.py"

_ISOLATED_ENV_KEYS = (
    "SITE_ID",
    "SITE_NAME",
    "BOT_PLATFORM",
    "GAME_ADMIN_KEY",
    "HERMES_ADAPTER_KEY",
    "ONEBOT_TOKEN",
    "ONEBOT_WEBHOOK_SECRET",
    "TG_BOT_TOKEN",
    "TELEGRAM_BOT_TOKEN",
    "TOKEN_BENEFIT_BOT_TOKEN",
    "TG_WEBHOOK_SECRET",
    "TELEGRAM_WEBHOOK_SECRET",
    "CHATOPS_WEBHOOK_SECRET",
    "NEWAPI_CHATOPS_SECRET",
    "QQ_BRIDGE_SECRET",
    "CHATOPS_ADMIN_EXTERNAL_IDS",
    "CHATOPS_ADMIN_IDS",
    "CHATOPS_ADMIN_QQ_IDS",
    "CHATOPS_ADMIN_TG_IDS",
    "HERMES_ADAPTER_LOG",
    "HERMES_BURN_QUEUE_CAPACITY",
    "HERMES_GAME_CONFIG_CACHE",
    "OPENAI_BASE_URL",
    "OPENAI_API_KEY",
    "IMAGE_API_BASE_URL",
    "IMAGE_OPENAI_BASE_URL",
    "IMAGE_API_KEY",
    "IMAGE_OPENAI_API_KEY",
    "IMAGE_MODEL",
    "IMAGE_SIZE",
    "IMAGE_TIMEOUT",
    "IMAGE_RETRY_LIMIT",
    "IMAGE_RETRY_BASE_DELAY",
    "IMAGE_RETRY_MAX_DELAY",
)


@pytest.fixture
def adapter_loader(monkeypatch, tmp_path):
    loaded = []

    def load(**env):
        for key in _ISOLATED_ENV_KEYS:
            monkeypatch.delenv(key, raising=False)
        defaults = {
            "SITE_ID": "prox-test",
            "SITE_NAME": "prox-test",
            "BOT_PLATFORM": "qq",
            "GAME_ADMIN_KEY": "planner-secret",
            "HERMES_ADAPTER_KEY": "",
            "ONEBOT_TOKEN": "",
            "ONEBOT_WEBHOOK_SECRET": "onebot-secret",
            "TG_BOT_TOKEN": "",
            "TG_WEBHOOK_SECRET": "telegram-secret",
            "TELEGRAM_WEBHOOK_SECRET": "",
            "CHATOPS_WEBHOOK_SECRET": "internal-chatops-secret",
            "NEWAPI_CHATOPS_SECRET": "",
            "QQ_BRIDGE_SECRET": "",
            "CHATOPS_ADMIN_EXTERNAL_IDS": "",
            "CHATOPS_ADMIN_IDS": "",
            "CHATOPS_ADMIN_QQ_IDS": "",
            "CHATOPS_ADMIN_TG_IDS": "",
            "HERMES_ADAPTER_LOG": str(tmp_path / "adapter.log"),
            "HERMES_BURN_QUEUE_CAPACITY": "8",
            "OPENAI_BASE_URL": "",
            "OPENAI_API_KEY": "",
        }
        defaults.update(env)
        for key, value in defaults.items():
            if value is None:
                monkeypatch.delenv(key, raising=False)
            else:
                monkeypatch.setenv(key, str(value))

        module_name = f"prox_hermes_adapter_test_{len(loaded)}_{id(monkeypatch)}"
        spec = importlib.util.spec_from_file_location(module_name, ADAPTER_PATH)
        module = importlib.util.module_from_spec(spec)
        sys.path.insert(0, str(SITE_ROOT))
        try:
            assert spec.loader is not None
            spec.loader.exec_module(module)
        finally:
            sys.path.pop(0)
        loaded.append((module_name, module))
        return module

    yield load

    for module_name, module in reversed(loaded):
        module._stop_burn_scheduler(clear_pending=True)
        module._ASYNC_EXECUTOR.shutdown(wait=False, cancel_futures=True)
        sys.modules.pop(module_name, None)


class _FakeRequest:
    def __init__(self, path="/onebot", headers=None, payload=None):
        body = json.dumps(payload if payload is not None else {}).encode("utf-8")
        self.path = path
        self.command = "POST"
        self.headers = dict(headers or {})
        self.headers.setdefault("Content-Length", str(len(body)))
        self.rfile = io.BytesIO(body)
        self.wfile = io.BytesIO()
        self.status = None
        self.response_headers = []

    def send_response(self, status):
        self.status = status

    def send_header(self, name, value):
        self.response_headers.append((name, value))

    def end_headers(self):
        return None


class _FakeHTTPResponse:
    status = 200

    def __init__(self, payload):
        self._body = json.dumps(payload).encode("utf-8")

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def read(self, _limit=None):
        return self._body


def _post(adapter, path, payload, headers=None):
    request = _FakeRequest(path=path, headers=headers, payload=payload)
    adapter.Handler.do_POST(request)
    response = json.loads(request.wfile.getvalue() or b"{}")
    return request, response


def _onebot_signature(secret, payload):
    body = json.dumps(payload).encode("utf-8")
    digest = hmac.new(secret.encode("utf-8"), body, hashlib.sha1).hexdigest()
    return f"sha1={digest}"


def test_admin_matching_uses_only_immutable_platform_user_id(adapter_loader):
    adapter = adapter_loader(
        CHATOPS_ADMIN_EXTERNAL_IDS="generic-admin",
        CHATOPS_ADMIN_QQ_IDS="4242",
        CHATOPS_ADMIN_TG_IDS="8484",
    )

    assert adapter._is_source_admin("qq", "4242", "not-an-admin") is True
    assert adapter._is_source_admin("tg", "8484", "not-an-admin") is True
    assert adapter._is_source_admin("qq", "ordinary", "4242", "owner") is False
    assert adapter._is_source_admin("tg", "ordinary", "@8484", "administrator") is False
    assert adapter._is_source_admin("qq", "generic-admin", "someone") is True


def test_planner_fails_closed_when_adapter_key_is_missing(adapter_loader):
    adapter = adapter_loader(GAME_ADMIN_KEY="", HERMES_ADAPTER_KEY="")

    request, response = _post(
        adapter,
        "/v1/chatops/plan",
        {"message": "查我的额度", "source": "qq", "user_id": "1001"},
    )

    assert request.status == 401
    assert response["error"] == "unauthorized"


def test_game_admin_auth_is_fail_closed_and_rejects_query_keys(adapter_loader):
    unconfigured = adapter_loader(GAME_ADMIN_KEY="", HERMES_ADAPTER_KEY="")
    assert not unconfigured.game_admin_authorized(_FakeRequest(path="/game-admin"))

    configured = adapter_loader(GAME_ADMIN_KEY="admin-test-key")
    assert not configured.game_admin_authorized(
        _FakeRequest(path="/game-admin?key=admin-test-key")
    )
    assert not configured.game_admin_authorized(
        _FakeRequest(path="/game-admin", headers={"X-GameAdmin-Key": "wrong"})
    )
    assert configured.game_admin_authorized(
        _FakeRequest(path="/game-admin", headers={"X-GameAdmin-Key": "admin-test-key"})
    )


def test_internal_chatops_secret_is_sent_in_headers_not_url(adapter_loader):
    adapter = adapter_loader()
    url = adapter._chatops_url(
        "http://127.0.0.1:3000",
        "/api/agent/chatops/action",
        "internal-secret",
        "qq",
    )
    headers = adapter._chatops_headers("internal-secret", "qq")
    tg_headers = adapter._chatops_headers("internal-secret", "tg")

    assert url == "http://127.0.0.1:3000/api/agent/chatops/action?source=qq"
    assert "internal-secret" not in url
    assert headers["Authorization"] == "Bearer internal-secret"
    assert tg_headers["Authorization"] == "Bearer internal-secret"
    assert tg_headers["X-Telegram-Bot-Api-Secret-Token"] == "internal-secret"


def test_system_config_uses_configured_state_path(adapter_loader, tmp_path):
    config_path = tmp_path / "runtime" / "game_config.json"
    config_path.parent.mkdir()
    config_path.write_text(
        json.dumps({"system": {"group_risk_enabled": False, "burn_after_seconds": 9}}),
        encoding="utf-8",
    )

    adapter = adapter_loader(HERMES_GAME_CONFIG_CACHE=str(config_path))

    assert adapter.GAME_CONFIG_PATH == str(config_path)
    assert adapter._load_system_config() == {
        "group_risk_enabled": False,
        "burn_after_seconds": 9,
    }


def test_planner_recomputes_admin_and_allows_only_self_quota(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader(GAME_ADMIN_KEY="planner-secret")
    captured = {}

    def fake_model(payload):
        captured.update(payload)
        return {
            "reply": "已撤回目标消息，其他操作也已完成",
            "actions": [
                {"type": "user.quota.read", "payload": {}},
                {
                    "type": "user.quota.read",
                    "payload": {"target_external_id": "ordinary-user"},
                },
                {
                    "type": "user.quota.read",
                    "payload": {"target_external_id": "someone-else"},
                },
                {"type": "user.quota.read", "new_api_user_id": 17},
                {"type": "site.state.read", "payload": {}},
                {"type": "group.message.delete", "payload": {"target_message_id": "9"}},
                {"type": "reward.grant.small", "quota_amount": 500000},
            ],
        }

    monkeypatch.setattr(adapter, "handle_with_games", lambda payload: None)
    monkeypatch.setattr(adapter, "call_model", fake_model)
    payload = {
        "message": "帮我撤回消息，再查额度",
        "source": "qq",
        "user_id": "ordinary-user",
        "username": "ordinary-user",
        "is_admin": True,
        "issuer_role": "owner",
    }

    request, response = _post(
        adapter,
        "/v1/chatops/plan",
        payload,
        headers={"X-Hermes-Key": "planner-secret"},
    )

    assert request.status == 200
    assert captured["is_admin"] is False
    assert captured["_trusted_is_admin"] is False
    actions = response["result"]["actions"]
    assert [action["type"] for action in actions] == [
        "user.quota.read",
        "user.quota.read",
    ]
    assert response["result"]["notes"] == "untrusted_admin_claim_or_action_blocked"
    for optimistic_copy in ("已撤回", "已完成", "搞定", "处理好了"):
        assert optimistic_copy not in response["result"]["reply"]


def test_fully_blocked_member_actions_replace_optimistic_reply(adapter_loader):
    adapter = adapter_loader()
    result = adapter._guard_llm_result(
        {"message": "替我操作", "user_id": "member-1", "_trusted_is_admin": False},
        {
            "reply": "搞定，消息已撤回，任务已完成",
            "actions": [
                {
                    "type": "user.quota.read",
                    "payload": {"target_external_id": "member-2"},
                },
                {"type": "message.qq.send", "payload": {"text": "done"}},
            ],
        },
    )

    assert result["actions"] == []
    assert "普通成员" in result["reply"]
    for optimistic_copy in ("已撤回", "已完成", "搞定", "处理好了"):
        assert optimistic_copy not in result["reply"]


def test_onebot_webhook_accepts_standard_bearer_and_dedicated_header(adapter_loader):
    adapter = adapter_loader(
        ONEBOT_WEBHOOK_SECRET="expected-secret",
        GAME_ADMIN_KEY="admin-secret",
        HERMES_ADAPTER_KEY="",
    )
    payload = {"post_type": "meta_event"}

    rejected_variants = [
        {},
        {"X-OneBot-Token": "wrong-secret"},
        {"Authorization": "Bearer wrong-secret"},
        {"Authorization": "Basic expected-secret"},
        {"Authorization": "Bearer admin-secret"},
        {"X-Access-Token": "expected-secret"},
        {"X-Hermes-Key": "expected-secret"},
        {"X-GameAdmin-Key": "expected-secret"},
    ]
    for headers in rejected_variants:
        request, response = _post(adapter, "/onebot", payload, headers=headers)
        assert request.status == 401
        assert response["error"] == "unauthorized_webhook"

    request, response = _post(adapter, "/onebot?token=expected-secret", payload)
    assert request.status == 401
    assert response["error"] == "unauthorized_webhook"

    accepted_variants = [
        {"X-OneBot-Token": "expected-secret"},
        {"Authorization": "Bearer expected-secret"},
    ]
    for headers in accepted_variants:
        request, _ = _post(adapter, "/onebot", payload, headers=headers)
        assert request.status == 200


def test_onebot_webhook_accepts_napcat_hmac_signature_and_rejects_tampering(
    adapter_loader,
):
    adapter = adapter_loader(ONEBOT_WEBHOOK_SECRET="expected-secret")
    payload = {"post_type": "meta_event", "time": 12345}

    accepted, _ = _post(
        adapter,
        "/onebot",
        payload,
        headers={"X-Signature": _onebot_signature("expected-secret", payload)},
    )
    assert accepted.status == 200

    rejected_variants = [
        "sha1=" + "0" * 40,
        _onebot_signature("wrong-secret", payload),
        "md5=" + "0" * 32,
        "sha1=not-hex",
    ]
    for signature in rejected_variants:
        request, response = _post(
            adapter,
            "/onebot",
            payload,
            headers={"X-Signature": signature},
        )
        assert request.status == 401
        assert response["error"] == "unauthorized_webhook"

    tampered_payload = {**payload, "time": 54321}
    request, response = _post(
        adapter,
        "/onebot",
        tampered_payload,
        headers={"X-Signature": _onebot_signature("expected-secret", payload)},
    )
    assert request.status == 401
    assert response["error"] == "unauthorized_webhook"


def test_onebot_webhook_secret_falls_back_to_onebot_api_token(adapter_loader):
    adapter = adapter_loader(
        ONEBOT_WEBHOOK_SECRET="",
        ONEBOT_TOKEN="shared-onebot-secret",
    )

    assert adapter.ONEBOT_WEBHOOK_SECRET == "shared-onebot-secret"
    request, _ = _post(
        adapter,
        "/onebot",
        {"post_type": "meta_event"},
        headers={"Authorization": "Bearer shared-onebot-secret"},
    )
    assert request.status == 200


def test_telegram_webhook_accepts_only_its_dedicated_header(adapter_loader):
    adapter = adapter_loader(TG_WEBHOOK_SECRET="expected-secret")
    payload = {"update_id": 9001}

    rejected_variants = [
        {},
        {"X-Telegram-Bot-Api-Secret-Token": "wrong-secret"},
        {"Authorization": "Bearer expected-secret"},
        {"X-Hermes-Webhook-Secret": "expected-secret"},
        {"X-Access-Token": "expected-secret"},
    ]
    for headers in rejected_variants:
        request, response = _post(
            adapter, "/telegram/webhook", payload, headers=headers
        )
        assert request.status == 401
        assert response["error"] == "unauthorized_webhook"

    request, response = _post(
        adapter, "/telegram/webhook?token=expected-secret", payload
    )
    assert request.status == 401
    assert response["error"] == "unauthorized_webhook"

    request, _ = _post(
        adapter,
        "/telegram/webhook",
        payload,
        headers={"X-Telegram-Bot-Api-Secret-Token": "expected-secret"},
    )
    assert request.status == 200


@pytest.mark.parametrize(
    ("path", "secret_env", "payload"),
    [
        ("/onebot", "ONEBOT_WEBHOOK_SECRET", {"post_type": "meta_event"}),
        ("/telegram/webhook", "TG_WEBHOOK_SECRET", {"update_id": 9002}),
    ],
)
def test_webhooks_fail_closed_with_empty_configuration(
    adapter_loader, path, secret_env, payload
):
    empty_config = {secret_env: ""}
    if secret_env == "TG_WEBHOOK_SECRET":
        empty_config.update(TELEGRAM_WEBHOOK_SECRET="", CHATOPS_WEBHOOK_SECRET="")
    adapter = adapter_loader(**empty_config)

    assert (
        adapter.ONEBOT_WEBHOOK_SECRET
        if secret_env == "ONEBOT_WEBHOOK_SECRET"
        else adapter.TG_WEBHOOK_SECRET
    ) == ""

    request, response = _post(
        adapter, path, payload, headers={"Authorization": "Bearer anything"}
    )

    assert request.status == 401
    assert response["error"] == "unauthorized_webhook"


def test_telegram_handler_replay_is_atomic_and_prefers_update_id(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader(TG_WEBHOOK_SECRET="telegram-secret")
    adapter._PROCESSED_MIDS.clear()
    side_effects = []
    side_effect_lock = threading.Lock()

    def record_side_effect(name):
        with side_effect_lock:
            side_effects.append(name)

    monkeypatch.setattr(
        adapter, "_tg_notice_event", lambda update: record_side_effect("notice")
    )
    monkeypatch.setattr(
        adapter,
        "handle_tg_message",
        lambda update: record_side_effect("message") or {"ok": True, "handled": True},
    )
    payload = {
        "update_id": 123456,
        "message": {
            "message_id": 77,
            "chat": {"id": -10001, "type": "supergroup"},
            "from": {"id": 501},
            "text": "hello",
        },
    }
    worker_count = 32
    barrier = threading.Barrier(worker_count)
    statuses = []

    def replay():
        barrier.wait()
        request, _ = _post(
            adapter,
            "/telegram/webhook",
            payload,
            headers={"X-Telegram-Bot-Api-Secret-Token": "telegram-secret"},
        )
        with side_effect_lock:
            statuses.append(request.status)

    threads = [threading.Thread(target=replay) for _ in range(worker_count)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join(timeout=3)

    assert not any(thread.is_alive() for thread in threads)
    assert statuses == [200] * worker_count
    assert side_effects.count("notice") == 1
    assert side_effects.count("message") == 1


def test_telegram_fallback_event_key_expires_after_ttl(adapter_loader, monkeypatch):
    adapter = adapter_loader(TG_WEBHOOK_SECRET="telegram-secret")
    adapter._PROCESSED_MIDS.clear()
    now = [1000.0]
    calls = []
    monkeypatch.setattr(adapter.time, "time", lambda: now[0])
    monkeypatch.setattr(adapter, "_tg_notice_event", lambda update: None)
    monkeypatch.setattr(
        adapter,
        "handle_tg_message",
        lambda update: calls.append(update) or {"ok": True},
    )
    payload = {
        "message": {
            "message_id": 88,
            "chat": {"id": -20002, "type": "supergroup"},
            "from": {"id": 601},
            "text": "fallback key",
        }
    }
    headers = {"X-Telegram-Bot-Api-Secret-Token": "telegram-secret"}

    _post(adapter, "/telegram/webhook", payload, headers=headers)
    _post(adapter, "/telegram/webhook", payload, headers=headers)
    assert len(calls) == 1

    now[0] += adapter._PROCESSED_MID_TTL + 1
    _post(adapter, "/telegram/webhook", payload, headers=headers)
    assert len(calls) == 2


def test_game_settlement_idempotency_is_stable_for_event_key(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader(CHATOPS_WEBHOOK_SECRET="internal-secret")
    requests = []

    def fake_urlopen(request, timeout=0):
        requests.append(json.loads(request.data.decode("utf-8")))
        return _FakeHTTPResponse({"ok": True, "data": {"success": True}})

    monkeypatch.setattr(adapter.urllib.request, "urlopen", fake_urlopen)
    monkeypatch.setattr(
        adapter, "_peek_game_commission_events", lambda source, actions: (0, [])
    )
    monkeypatch.setattr(adapter, "_commit_game_commission_events", lambda events: 0)
    actions = [
        {
            "type": "reward.grant.small",
            "user_id": 17,
            "target_external_id": "tg-user-17",
            "quota_amount": 500000,
            "budget_pool": "game",
            "reason": "dice_win",
        }
    ]

    first = adapter.run_game_settlement(
        actions, "tg", "-30003", "tg-user-17", event_key="tg:update:7001"
    )
    monkeypatch.setattr(adapter.time, "time", lambda: 9999999999.0)
    second = adapter.run_game_settlement(
        actions, "tg", "-30003", "tg-user-17", event_key="tg:update:7001"
    )
    adapter.run_game_settlement(
        actions, "tg", "-30003", "tg-user-17", event_key="tg:update:7002"
    )

    keys = [item["action"]["idempotency_key"] for item in requests]
    assert keys[0] == keys[1]
    assert keys[0] != keys[2]
    assert first[0]["settlement_id"] == second[0]["settlement_id"] == keys[0]
    assert "9999999999" not in keys[0]


def test_game_director_reward_pipeline_remains_trusted(adapter_loader, monkeypatch):
    adapter = adapter_loader()
    captured = []
    monkeypatch.setattr(
        adapter,
        "run_game_settlement",
        lambda actions,
        source,
        room_id,
        external_user_id,
        event_key="": captured.append(
            (actions, source, room_id, external_user_id, event_key)
        )
        or [
            {
                "ok": True,
                "action_type": "reward.settlement.batch",
                "settlement_id": "stable",
            }
        ],
    )
    reward = {
        "type": "reward.grant.small",
        "user_id": 17,
        "quota_amount": 100,
        "reason": "dice_win",
    }

    result = adapter.run_game_action_pipeline(
        [reward], "tg", "room", "user", event_key="tg:update:1"
    )

    assert len(captured) == 1
    assert captured[0][4] == "tg:update:1"
    assert result["results"][0]["ok"] is True


def test_failed_game_settlement_removes_optimistic_copy(adapter_loader):
    adapter = adapter_loader()
    result = {"reply": "恭喜获奖，奖励已经到账！", "notes": "winner"}
    action_results = [
        {
            "ok": False,
            "action_type": "reward.settlement.batch",
            "error": "database unavailable",
            "settlement_id": "settle-12345678",
        }
    ]

    finalized = adapter._finalize_game_reply(result, action_results)

    assert "恭喜" not in finalized["reply"]
    assert "获奖" not in finalized["reply"]
    assert "已经到账" not in finalized["reply"]
    assert "结算失败" in finalized["reply"]
    assert finalized["notes"] == "game_settlement_failed"


def test_non_settlement_failure_does_not_pollute_settlement_copy(adapter_loader):
    adapter = adapter_loader()
    result = {"reply": "本轮游戏已结束", "notes": "round_done"}
    action_results = [
        {
            "ok": False,
            "action_type": "message.tg.send",
            "error": "telegram unavailable",
        },
        {
            "ok": True,
            "action_type": "reward.settlement.batch",
            "settlement_id": "settle-stable-0001",
        },
    ]

    finalized = adapter._finalize_game_reply(result, action_results)

    assert finalized["notes"] == "round_done"
    assert "额度结算失败" not in finalized["reply"]
    assert "额度结算：已完成" in finalized["reply"]
    assert "telegram unavailable" not in finalized["reply"]


def test_burn_scheduler_is_bounded_and_does_not_use_async_worker(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader()
    deleted = []
    monkeypatch.setattr(
        adapter, "onebot_delete_msg", lambda mid: deleted.append(str(mid)) or True
    )
    monkeypatch.setattr(
        adapter,
        "_submit_background",
        lambda *args, **kwargs: pytest.fail(
            "burn delay must not occupy the general async worker"
        ),
    )
    adapter._reset_burn_scheduler_for_tests(capacity=2, start_worker=False)
    try:
        assert adapter.schedule_burn(["1"], delay=30) is True
        assert adapter.schedule_burn(["2"], delay=30) is True
        assert adapter.schedule_burn(["3"], delay=30) is False
        assert adapter._burn_scheduler_snapshot()["queued"] == 2
        assert deleted == []
    finally:
        adapter._stop_burn_scheduler(clear_pending=True)
    assert adapter._burn_scheduler_snapshot()["worker_alive"] is False


def test_burn_scheduler_uses_one_worker_executes_jobs_and_stops(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader()
    deleted = []
    monkeypatch.setattr(
        adapter, "onebot_delete_msg", lambda mid: deleted.append(str(mid)) or True
    )
    adapter._reset_burn_scheduler_for_tests(capacity=4, start_worker=False)
    first_worker = None

    try:
        assert adapter.schedule_burn(["11"], delay=0.02) is True
        first_worker = adapter._BURN_WORKER
        assert adapter.schedule_burn(["12"], delay=0.03) is True
        assert adapter._BURN_WORKER is first_worker

        deadline = time.monotonic() + 1
        while len(deleted) < 2 and time.monotonic() < deadline:
            time.sleep(0.01)

        assert deleted == ["11", "12"]
        assert adapter._burn_scheduler_snapshot()["queued"] == 0
    finally:
        adapter._stop_burn_scheduler(clear_pending=True, join_timeout=1.0)
    assert first_worker is not None
    assert not first_worker.is_alive()


def test_bind_burn_queue_overload_withdraws_synchronously_and_logs(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader(ONEBOT_WEBHOOK_SECRET="onebot-secret")
    adapter._reset_burn_scheduler_for_tests(capacity=1, start_worker=False)
    sync_deleted = []
    logs = []
    monkeypatch.setattr(adapter, "log", lambda event: logs.append(event))
    monkeypatch.setattr(
        adapter, "onebot_delete_msg", lambda mid: sync_deleted.append(str(mid)) or True
    )
    monkeypatch.setattr(
        adapter,
        "confirm_bind_via_newapi",
        lambda *args: {"ok": True, "new_api_user_id": 17},
    )
    monkeypatch.setattr(adapter, "send_qq_reply", lambda *args: "reply-901")
    monkeypatch.setattr(
        adapter, "_trusted_qq_admin", lambda room_id, user_id: (False, "member")
    )
    monkeypatch.setattr(
        adapter, "_capture_chatops_message", lambda *args, **kwargs: None
    )
    monkeypatch.setattr(adapter, "RISK_ENABLED", False)

    try:
        assert adapter.schedule_burn(["queue-occupied"], delay=60) is True
        payload = {
            "post_type": "message",
            "message_type": "group",
            "group_id": 12345,
            "user_id": 67890,
            "message_id": 901,
            "message": "绑定 ABCD2345",
            "raw_message": "绑定 ABCD2345",
            "sender": {"card": "member", "role": "member"},
        }
        request, response = _post(
            adapter,
            "/onebot",
            payload,
            headers={"X-OneBot-Token": "onebot-secret"},
        )

        assert request.status == 200
        assert response["bind_handled"] is True
        assert sync_deleted == ["reply-901", "901"]
        fallback_logs = [
            event
            for event in logs
            if event.get("event") == "sensitive_burn_sync_fallback"
        ]
        assert len(fallback_logs) == 1
        assert fallback_logs[0]["message_ids"] == ["reply-901", "901"]
    finally:
        adapter._stop_burn_scheduler(clear_pending=True)
    assert adapter._burn_scheduler_snapshot()["worker_alive"] is False


def test_image_prompt_detection_accepts_commands_without_false_positives(
    adapter_loader,
):
    adapter = adapter_loader()

    assert (
        adapter.detect_image_prompt(
            "\u751f\u6210\u897f\u6e38\u8bb0\u4efb\u52a1\u62c6\u89e34k\u56fe\u7247"
        )
        == "\u897f\u6e38\u8bb0\u4efb\u52a1\u62c6\u89e34k"
    )
    assert (
        adapter.detect_image_prompt(
            "\u751f\u56fe\uff1a\u897f\u6e38\u8bb0\u4eba\u7269\u62c6\u89e3\uff0c4k"
        )
        == "\u897f\u6e38\u8bb0\u4eba\u7269\u62c6\u89e3\uff0c4k"
    )
    assert (
        adapter.detect_image_prompt("\u753b\u56fe\u753b\u7279\u6717\u666e")
        == "\u753b\u7279\u6717\u666e"
    )
    assert (
        adapter.detect_image_prompt("\u751f\u56fe\u7684\u9700\u6c42\u5f88\u591a") == ""
    )
    assert (
        adapter.detect_image_prompt("\u8fd9\u5f20\u56fe\u753b\u5f97\u4e0d\u9519") == ""
    )


def test_image_service_retries_transient_524_and_returns_generated_url(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader(
        IMAGE_API_BASE_URL="https://images.example.test/v1",
        IMAGE_API_KEY="image-test-key",
        IMAGE_MODEL="gpt-image-2",
        IMAGE_RETRY_LIMIT="2",
        IMAGE_RETRY_BASE_DELAY="0",
        IMAGE_RETRY_MAX_DELAY="0",
    )
    requests = []

    def fake_urlopen(request, timeout=0):
        requests.append((request, timeout))
        if len(requests) == 1:
            body = io.BytesIO(
                json.dumps(
                    {
                        "error": {
                            "message": "temporary upstream timeout",
                            "code": "upstream_timeout",
                            "account_email": "must-not-be-logged@example.test",
                        }
                    }
                ).encode("utf-8")
            )
            raise urllib.error.HTTPError(
                request.full_url,
                524,
                "timeout",
                {"Retry-After": "0"},
                body,
            )
        return _FakeHTTPResponse(
            {"data": [{"url": "https://cdn.example.test/generated.png"}]}
        )

    monkeypatch.setattr(adapter.urllib.request, "urlopen", fake_urlopen)
    monkeypatch.setattr(
        adapter.time,
        "sleep",
        lambda seconds: pytest.fail(f"zero-delay retry slept for {seconds}"),
    )

    result = adapter._call_image_service("\u4e00\u53ea\u767d\u8272\u5496\u5561\u676f")

    assert result == {
        "ok": True,
        "photo_url": "https://cdn.example.test/generated.png",
        "revised_prompt": "",
    }
    assert len(requests) == 2
    assert (
        requests[0][0].full_url == "https://images.example.test/v1/images/generations"
    )
    sent_payload = json.loads(requests[0][0].data)
    assert sent_payload["model"] == "gpt-image-2"
    assert sent_payload["response_format"] == "url"


def test_image_service_does_not_retry_auth_failure_and_redacts_upstream_identity(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader(
        IMAGE_API_BASE_URL="https://images.example.test/v1",
        IMAGE_API_KEY="invalid-image-key",
        IMAGE_RETRY_LIMIT="3",
        IMAGE_RETRY_BASE_DELAY="0",
    )
    calls = []

    def fake_urlopen(request, timeout=0):
        calls.append(request)
        body = io.BytesIO(
            json.dumps(
                {
                    "error": {
                        "message": "invalid credential",
                        "code": "invalid_api_key",
                        "account_email": "private@example.test",
                    }
                }
            ).encode("utf-8")
        )
        raise urllib.error.HTTPError(request.full_url, 401, "unauthorized", {}, body)

    monkeypatch.setattr(adapter.urllib.request, "urlopen", fake_urlopen)

    result = adapter._call_image_service("\u6d4b\u8bd5\u56fe\u7247")

    assert result["ok"] is False
    assert result["error"].startswith("http_401:")
    assert "invalid_api_key" in result["error"]
    assert "private@example.test" not in result["error"]
    assert "account_email" not in result["error"]
    assert len(calls) == 1


def test_image_job_reports_delivery_failure_and_keeps_generated_link(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader()
    replies = []
    logs = []
    adapter._set_image_pending(
        "qq",
        "925249987",
        "1001",
        "\u4e00\u53ea\u767d\u8272\u5496\u5561\u676f",
    )

    monkeypatch.setattr(
        adapter,
        "_call_image_service",
        lambda prompt: {
            "ok": True,
            "photo_url": "https://cdn.example.test/generated.png",
            "revised_prompt": "",
        },
    )
    monkeypatch.setattr(adapter, "send_qq_photo", lambda *args, **kwargs: None)
    monkeypatch.setattr(
        adapter,
        "send_qq_reply",
        lambda *args: replies.append(args) or "fallback-message-id",
    )
    monkeypatch.setattr(adapter, "log", lambda event: logs.append(event))

    def run_inline(job, **_kwargs):
        job()
        return True

    monkeypatch.setattr(adapter, "_submit_background", run_inline)

    adapter._start_qq_image_job(
        "group",
        "925249987",
        "925249987",
        "1001",
        "\u6d4b\u8bd5\u7528\u6237",
        "\u4e00\u53ea\u767d\u8272\u5496\u5561\u676f",
    )

    assert len(replies) == 1
    assert "https://cdn.example.test/generated.png" in replies[0][2]
    image_logs = [event for event in logs if event.get("event") == "image_generate"]
    assert image_logs[-1]["status"] == "delivery_error"
    assert adapter._get_image_pending("qq", "925249987", "1001") is None


def test_image_delivery_fallback_does_not_expose_base64_payload(adapter_loader):
    adapter = adapter_loader()

    message = adapter._image_delivery_fallback_message(
        "tester", "base64://private-image-payload", "private"
    )

    assert "private-image-payload" not in message
    assert "QQ" in message


def test_image_worker_error_is_reported_redacted_and_clears_pending(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader(IMAGE_API_KEY="image-test-key")
    replies = []
    logs = []
    adapter._set_image_pending("qq", "room-1", "user-1", "draw a dashboard")

    def fail_image_service(prompt):
        raise RuntimeError("worker failed for user@example.com")

    monkeypatch.setattr(adapter, "_call_image_service", fail_image_service)
    monkeypatch.setattr(
        adapter,
        "send_qq_reply",
        lambda *args: replies.append(args) or "worker-error-message-id",
    )
    monkeypatch.setattr(adapter, "log", lambda event: logs.append(event))

    def run_inline(job, **_kwargs):
        job()
        return True

    monkeypatch.setattr(adapter, "_submit_background", run_inline)

    adapter._start_qq_image_job(
        "private", "user-1", "room-1", "user-1", "member", "draw a dashboard"
    )

    assert len(replies) == 1
    assert logs[-1]["status"] == "worker_error"
    assert logs[-1]["detail"] == "worker failed for <redacted-email>"
    assert adapter._get_image_pending("qq", "room-1", "user-1") is None
