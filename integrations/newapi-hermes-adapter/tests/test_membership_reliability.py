from test_prox_adapter_security import _FakeRequest, adapter_loader  # noqa: F401


def test_membership_endpoint_uses_canonical_newapi_base_url(adapter_loader):
    adapter = adapter_loader(
        NEWAPI_INTERNAL_BASE_URL="http://new-api.internal:3000",
        NEW_API_INTERNAL_BASE_URL=None,
        NEW_API_CHATOPS_BASE_URL=None,
    )
    assert adapter._membership_endpoint() == (
        "http://new-api.internal:3000/api/chat-membership/events"
    )


def test_membership_retry_queue_has_backoff_and_attempt_limit(
    adapter_loader, monkeypatch
):
    adapter = adapter_loader(
        NEW_API_MEMBERSHIP_RETRY_MAX_ATTEMPTS="3",
        NEW_API_MEMBERSHIP_RETRY_BASE_DELAY_SECONDS="10",
        NEW_API_MEMBERSHIP_RETRY_MAX_DELAY_SECONDS="60",
    )
    now, calls = [100.0], []
    monkeypatch.setattr(adapter.time, "time", lambda: now[0])

    def fail(payload):
        calls.append(payload)
        raise OSError("down")

    monkeypatch.setattr(adapter, "_post_membership_payload", fail)
    adapter._queue_membership_retry({"event_id": "event-100"}, attempts=1)
    assert adapter.flush_membership_retry_queue()["failed"] == 0
    now[0] = 110.0
    assert adapter.flush_membership_retry_queue()["failed"] == 1
    now[0] = 129.0
    assert adapter.flush_membership_retry_queue()["failed"] == 0
    now[0] = 130.0
    assert adapter.flush_membership_retry_queue()["dropped"] == 1
    assert len(calls) == 2 and adapter._membership_retry_queue == []


def test_membership_retry_does_not_block_new_event(adapter_loader, monkeypatch):
    adapter, scheduled, posted = adapter_loader(), [], []
    monkeypatch.setattr(
        adapter, "_schedule_membership_retry_flush", lambda: scheduled.append(True)
    )
    monkeypatch.setattr(
        adapter,
        "_post_membership_payload",
        lambda payload: posted.append(payload) or {"ok": True},
    )
    result = adapter.forward_membership_event(
        "qq", "room", "user", "join", event_at=100, event_id="event-100"
    )
    assert result == {"ok": True}
    assert scheduled == [True] and posted[0]["event_id"] == "event-100"


def test_qq_leave_notice_skips_missing_member_lookup(adapter_loader, monkeypatch):
    adapter, lookups = adapter_loader(), []
    monkeypatch.setattr(
        adapter, "resolve_identity_via_newapi", lambda *args: {"new_api_user_id": 0}
    )
    monkeypatch.setattr(
        adapter, "_fetch_qq_member_profile", lambda *args: lookups.append(args) or {}
    )
    monkeypatch.setattr(
        adapter, "forward_membership_event", lambda **kwargs: {"ok": True}
    )
    monkeypatch.setattr(adapter, "_notify_game_director_notice", lambda *a, **kw: None)
    adapter.handle_onebot_notice(
        _FakeRequest(),
        {
            "notice_type": "group_decrease",
            "group_id": 925249987,
            "user_id": 123456,
            "sub_type": "leave",
            "time": 100,
        },
    )
    assert lookups == []
