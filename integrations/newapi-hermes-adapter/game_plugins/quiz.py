"""Daily quiz backed by the New API PostgreSQL question bank."""

import json
import os

from chatops_client import chatops_request, normalize_source

from .base import GamePlugin, GameResponse


ANSWER_PREFIXES = ("答:", "答：", "答 ")
CHOICE_LABELS = tuple("ABCDEFGHIJ")


def _safe_int(value, default=0):
    try:
        return int(float(value))
    except (TypeError, ValueError):
        return default


class QuizGame(GamePlugin):
    name = "quiz"
    display_name = "每日答题"
    description = "题目由站点数据库题库统一抽取和判定，支持按用户或按群共享轮次"
    tier = "interactive"
    triggers = ["答题", "每日答题", "答题帮助", "quiz", "helpquiz"]
    default_config = {
        "enabled": True,
        "reward_quota": 100000,
        "max_per_user_day": 10,
        "cooldown_seconds": 0,
        "budget_pool": "activity",
        "question_scope": "per_user",
        "question_ttl_seconds": 600,
        "question_count": 0,
        "quiz_limit_per_group": 0,
        "max_winners_per_question": 0,
        "max_attempts_per_question": 2,
        "bank_code": "",
    }

    def __init__(self):
        super().__init__()
        self._round_cache = {}

    def _source(self, ctx):
        raw = (
            getattr(ctx, "platform", "")
            or os.environ.get("CHATOPS_SOURCE", "")
            or os.environ.get("BOT_PLATFORM", "qq")
        )
        return normalize_source(str(raw))

    def _scope_mode(self):
        scope = (
            str(self.config.get("question_scope", "per_user") or "per_user")
            .lower()
            .strip()
        )
        if scope in ("group", "per_group", "shared", "room"):
            return "per_group"
        return "per_user"

    def _user_key(self, ctx):
        return str(
            getattr(ctx, "user_id", "") or getattr(ctx, "username", "") or "anonymous"
        ).strip()

    def _cache_key(self, ctx):
        group_id = str(getattr(ctx, "group_id", "") or "dm").strip() or "dm"
        if self._scope_mode() == "per_group":
            return f"group::{group_id}"
        return f"user::{group_id}::{self._user_key(ctx)}"

    def _action(self, ctx, action_type, **values):
        source = self._source(ctx)
        body = {
            "source": source,
            "room_id": str(getattr(ctx, "group_id", "") or ""),
            "user_external_id": str(getattr(ctx, "user_id", "") or ""),
            "username": str(getattr(ctx, "username", "") or ""),
            "action": {"type": action_type, "game_code": "quiz"},
        }
        if getattr(ctx, "new_api_user_id", 0):
            body["action"]["new_api_user_id"] = int(
                getattr(ctx, "new_api_user_id", 0) or 0
            )
        body["action"].update(
            {key: value for key, value in values.items() if value is not None}
        )
        response = chatops_request(
            "/api/agent/chatops/action", body, source=source, timeout=10
        )
        return self._unwrap_action(response)

    @staticmethod
    def _unwrap_action(response):
        if not isinstance(response, dict) or not response.get("success"):
            message = ""
            if isinstance(response, dict):
                message = str(
                    response.get("message") or response.get("error") or ""
                ).strip()
            raise RuntimeError(message or "答题服务请求失败")
        action = response.get("data") or {}
        result = (
            action.get("result")
            or action.get("result_json")
            or action.get("ResultJson")
        )
        if isinstance(result, str):
            try:
                result = json.loads(result)
            except Exception:
                result = {"summary": result}
        if not isinstance(result, dict):
            result = {}
        if str(action.get("status") or "").lower() == "failed":
            raise RuntimeError(str(result.get("error") or "答题服务执行失败"))
        return result

    def _draw_values(self):
        values = {
            "scope_mode": self._scope_mode(),
            "question_ttl_seconds": _safe_int(
                self.config.get("question_ttl_seconds"), 600
            ),
            "max_attempts_per_question": _safe_int(
                self.config.get("max_attempts_per_question"), 2
            ),
            "max_winners_per_question": _safe_int(
                self.config.get("max_winners_per_question"), 0
            ),
            "max_per_user_day": _safe_int(self.config.get("max_per_user_day"), 10),
            "quiz_limit_per_group": _safe_int(
                self.config.get("quiz_limit_per_group")
                or self.config.get("question_count"),
                0,
            ),
            "reward_quota": _safe_int(self.config.get("reward_quota"), 100000),
            "budget_pool": str(self.config.get("budget_pool") or "activity"),
            "close_on_correct": self.config.get("close_on_correct"),
        }
        bank_code = str(self.config.get("bank_code") or "").strip()
        bank_id = _safe_int(self.config.get("bank_id"), 0)
        if bank_code:
            values["bank_code"] = bank_code
        if bank_id > 0:
            values["bank_id"] = bank_id
        return values

    def _cache_round(self, ctx, result):
        if not isinstance(result, dict) or not result.get("active"):
            self._round_cache.pop(self._cache_key(ctx), None)
            return None
        question = result.get("question") or {}
        options = list(question.get("options") or [])
        if not str(question.get("prompt") or "").strip() or len(options) < 2:
            self._round_cache.pop(self._cache_key(ctx), None)
            return None
        cached = {
            "draw_id": _safe_int(result.get("draw_id"), 0),
            "round_key": str(result.get("round_key") or ""),
            "scope_mode": str(result.get("scope_mode") or self._scope_mode()),
            "expires_at": _safe_int(result.get("expires_at"), 0),
            "question": {
                "id": question.get("id"),
                "external_key": question.get("external_key"),
                "prompt": str(question.get("prompt") or "").strip(),
                "options": options,
                "difficulty": question.get("difficulty"),
            },
            "entry": result.get("entry") or {},
        }
        self._round_cache[self._cache_key(ctx)] = cached
        return cached

    def _load_round(self, ctx):
        result = self._action(ctx, "quiz.round.load", scope_mode=self._scope_mode())
        return self._cache_round(ctx, result)

    def _draw_round(self, ctx):
        result = self._action(ctx, "quiz.question.draw", **self._draw_values())
        return self._cache_round(ctx, result)

    def _cached_round(self, ctx):
        return self._round_cache.get(self._cache_key(ctx))

    @staticmethod
    def _answer_payload(text):
        text = str(text or "").strip()
        for prefix in ANSWER_PREFIXES:
            if text.startswith(prefix):
                return text[len(prefix) :].strip()
        if len(text) == 2 and text[0] == "答" and text[1].upper() in CHOICE_LABELS:
            return text[1:].strip()
        return None

    def _answer_index(self, text, cached):
        answer = self._answer_payload(text)
        if answer is None:
            answer = str(text or "").strip()
        upper = answer.upper()
        options = list((cached.get("question") or {}).get("options") or [])
        if upper in CHOICE_LABELS[: len(options)]:
            return CHOICE_LABELS.index(upper)
        for index, option in enumerate(options):
            if answer.casefold() == str(option).strip().casefold():
                return index
        return None

    def _is_answer(self, text, cached=None):
        payload = self._answer_payload(text)
        if payload is not None:
            return bool(payload)
        text = str(text or "").strip()
        if cached and self._answer_index(text, cached) is not None:
            return True
        return False

    @staticmethod
    def _is_prompt(text):
        lower = str(text or "").strip().lower()
        return lower.startswith(("答题", "每日答题", "quiz", "helpquiz"))

    def _format_round(self, cached, ctx):
        question = cached["question"]
        options = question["options"]
        option_lines = "\n".join(
            f"{CHOICE_LABELS[index]}. {option}" for index, option in enumerate(options)
        )
        scope_tip = ""
        if cached.get("scope_mode") != "per_group" and str(
            getattr(ctx, "group_id", "") or ""
        ):
            scope_tip = "\n👤 这是你的专属题目；其他成员发送「答题」会拿到自己的题目。"
        return (
            f"📝 当前题目: {question['prompt']}\n{option_lines}\n\n"
            "💡 可回复 A/B/C/D、答 A、答:答案内容，或答案原文"
            f"{scope_tip}"
        )

    @staticmethod
    def _friendly_error(error, username):
        message = str(error or "").strip()
        if "user daily limit reached" in message:
            limit = message.rsplit(":", 1)[-1].strip()
            return f"@{username} 今天答题次数已达上限（{limit} 次），明天再来！"
        if "group daily limit reached" in message:
            limit = message.rsplit(":", 1)[-1].strip()
            return f"@{username} 今天本群题目轮次已达上限（{limit} 题），明天再来！"
        if "no published questions" in message or "record not found" in message.lower():
            return "📝 当前没有已发布并绑定到本群的题库，请管理员在后台题库管理中发布题目。"
        return f"@{username} 答题服务暂时未完成本次操作：{message or 'unknown error'}"

    def match_followup(self, ctx):
        text = str(getattr(ctx, "text", "") or "").strip()
        cached = self._cached_round(ctx)
        if cached:
            return self._is_prompt(text) or self._is_answer(text, cached)
        return self._answer_payload(text) is not None

    def handle(self, ctx, sm, budget, escrow):
        text = str(getattr(ctx, "text", "") or "").strip()
        username = str(getattr(ctx, "username", "") or self._user_key(ctx) or "用户")
        cached = self._cached_round(ctx)

        if self._is_answer(text, cached) and not cached:
            try:
                cached = self._load_round(ctx)
            except Exception as error:
                return GameResponse.quick(self._friendly_error(error, username))

        if cached:
            if self._is_prompt(text):
                return GameResponse.quick(self._format_round(cached, ctx))
            answer_index = self._answer_index(text, cached)
            if answer_index is None:
                return None
            try:
                result = self._action(
                    ctx,
                    "quiz.answer.submit",
                    draw_id=cached.get("draw_id"),
                    round_key=cached.get("round_key"),
                    answer_index=answer_index,
                )
            except Exception as error:
                self._round_cache.pop(self._cache_key(ctx), None)
                return GameResponse.quick(self._friendly_error(error, username))

            if result.get("requires_binding"):
                return GameResponse.quick(
                    f"@{username} 答案正确；请先发送「验牌」绑定站点账号后再作答领奖。"
                )
            if result.get("correct"):
                self._round_cache.pop(self._cache_key(ctx), None)
                self.record_play(self._user_key(ctx))
                reward = _safe_int(result.get("reward_quota"), 0)
                answer = str(result.get("correct_answer") or "")
                reward_text = (
                    f" +${budget.quota_to_usd(reward):.2f}" if reward > 0 else ""
                )
                return GameResponse.quick(
                    f"@{username} 🎉 答对了！{reward_text}\n正确答案: {answer}"
                )

            remaining = _safe_int(result.get("remaining_attempts"), 0)
            if result.get("locked") or result.get("closed"):
                self._round_cache.pop(self._cache_key(ctx), None)
                self.record_play(self._user_key(ctx))
                answer = str(result.get("correct_answer") or "")
                return GameResponse.quick(
                    f"@{username} ❌ 这题作答机会已用完，正确答案是 {answer}\n重新发送「答题」领取下一题。"
                )
            cached.setdefault("entry", {})["attempts"] = _safe_int(
                result.get("attempts"), 0
            )
            return GameResponse.quick(
                f"@{username} ❌ 不对哦，再想想！你还有 {remaining} 次机会。"
            )

        if "帮助" in text or "help" in text.lower():
            return GameResponse.quick(
                "📝 发送「答题」领取数据库题库中的题目；作答支持 A/B/C/D、答 A、答:答案内容或答案原文。"
            )

        try:
            cached = self._draw_round(ctx)
        except Exception as error:
            return GameResponse.quick(self._friendly_error(error, username))
        if not cached:
            return GameResponse.quick(
                "📝 当前没有可用题目，请管理员检查题库发布和群绑定状态。"
            )
        return GameResponse(reply=self._format_round(cached, ctx), event="quiz_new")
