from __future__ import annotations

import sys
from pathlib import Path

import pytest


ADAPTER_ROOT = Path(__file__).resolve().parents[1]
if str(ADAPTER_ROOT) not in sys.path:
    sys.path.insert(0, str(ADAPTER_ROOT))

from game_plugins.base import GameContext  # noqa: E402
from game_plugins.quiz import QuizGame  # noqa: E402
import game_plugins.quiz as quiz_module  # noqa: E402


class _Budget:
    @staticmethod
    def quota_to_usd(quota):
        return round(int(quota or 0) / 500_000, 2)


def _context(text="答题"):
    return GameContext(
        site_id="prox-test",
        site_name="prox-test",
        platform="qq",
        group_id="room-1",
        user_id="qq-1001",
        username="tester",
        new_api_user_id=1001,
        is_bound=True,
        text=text,
    )


def _action_response(result, status="completed"):
    return {"success": True, "data": {"status": status, "result": result}}


def _draw_result():
    return {
        "ok": True,
        "active": True,
        "draw_id": 41,
        "round_key": "quiz:round-41",
        "scope_mode": "per_user",
        "expires_at": 9_999_999_999,
        "question": {
            "id": 7,
            "external_key": "general-7",
            "prompt": "2 + 2 = ?",
            "options": ["3", "4", "5"],
            "difficulty": "normal",
        },
        "entry": {"attempts": 0},
    }


def test_quiz_draw_uses_newapi_bank_and_caches_no_answer(monkeypatch):
    requests = []

    def fake_request(path, body, **kwargs):
        requests.append((path, body, kwargs))
        return _action_response(_draw_result())

    monkeypatch.setattr(quiz_module, "chatops_request", fake_request)
    game = QuizGame()
    game.on_load(
        {
            "bank_code": "general",
            "reward_quota": 120_000,
            "question_scope": "per_user",
            "max_attempts_per_question": 3,
        }
    )

    response = game.handle(_context(), None, _Budget(), None)

    assert "2 + 2 = ?" in response.reply
    assert "B. 4" in response.reply
    assert len(requests) == 1
    path, body, kwargs = requests[0]
    assert path == "/api/agent/chatops/action"
    assert kwargs["source"] == "qq"
    assert body["action"]["type"] == "quiz.question.draw"
    assert body["action"]["bank_code"] == "general"
    assert body["action"]["reward_quota"] == 120_000
    assert body["action"]["max_attempts_per_question"] == 3
    cached = game._round_cache["user::room-1::qq-1001"]
    assert "correct_answer" not in cached
    assert "correct_index" not in cached["question"]


def test_quiz_recovers_round_after_restart_and_submits_answer(monkeypatch):
    action_types = []

    def fake_request(_path, body, **_kwargs):
        action_type = body["action"]["type"]
        action_types.append(action_type)
        if action_type == "quiz.round.load":
            return _action_response(_draw_result())
        assert body["action"]["draw_id"] == 41
        assert body["action"]["round_key"] == "quiz:round-41"
        assert body["action"]["answer_index"] == 1
        return _action_response(
            {
                "ok": True,
                "correct": True,
                "reward_quota": 100_000,
                "correct_answer": "4",
            }
        )

    monkeypatch.setattr(quiz_module, "chatops_request", fake_request)
    game = QuizGame()

    response = game.handle(_context("答 B"), None, _Budget(), None)

    assert action_types == ["quiz.round.load", "quiz.answer.submit"]
    assert "答对了" in response.reply
    assert "正确答案: 4" in response.reply
    assert game._round_cache == {}


def test_quiz_keeps_round_for_retry_and_closes_when_locked(monkeypatch):
    outcomes = [
        {"ok": True, "correct": False, "attempts": 1, "remaining_attempts": 1},
        {
            "ok": True,
            "correct": False,
            "attempts": 2,
            "remaining_attempts": 0,
            "locked": True,
            "closed": True,
            "correct_answer": "4",
        },
    ]

    def fake_request(_path, body, **_kwargs):
        if body["action"]["type"] == "quiz.question.draw":
            return _action_response(_draw_result())
        return _action_response(outcomes.pop(0))

    monkeypatch.setattr(quiz_module, "chatops_request", fake_request)
    game = QuizGame()
    game.handle(_context(), None, _Budget(), None)

    retry = game.handle(_context("A"), None, _Budget(), None)
    assert "还有 1 次机会" in retry.reply
    assert game._round_cache

    locked = game.handle(_context("A"), None, _Budget(), None)
    assert "正确答案是 4" in locked.reply
    assert game._round_cache == {}


def test_quiz_binding_result_does_not_consume_cached_round(monkeypatch):
    def fake_request(_path, body, **_kwargs):
        if body["action"]["type"] == "quiz.question.draw":
            return _action_response(_draw_result())
        return _action_response({"ok": True, "correct": True, "requires_binding": True})

    monkeypatch.setattr(quiz_module, "chatops_request", fake_request)
    game = QuizGame()
    game.handle(_context(), None, _Budget(), None)

    response = game.handle(_context("答 B"), None, _Budget(), None)

    assert "先发送「验牌」" in response.reply
    assert game._round_cache


@pytest.mark.parametrize(
    ("message", "expected"),
    [
        ("quiz user daily limit reached: 3", "3 次"),
        ("quiz group daily limit reached: 8", "8 题"),
        ("quiz bank has no published questions", "没有已发布并绑定"),
    ],
)
def test_quiz_translates_backend_errors(message, expected):
    assert expected in QuizGame._friendly_error(message, "tester")
