#!/usr/bin/env python3
"""自进化学习引擎：质量追踪 => 周期性复盘 => 知识积累 => 动态注入。"""

from __future__ import annotations
import json, os, time, threading, urllib.request, urllib.error, traceback
from chatops_client import chatops_request

_STATE_DIR = os.environ.get("HERMES_STATE_DIR") or os.path.dirname(
    os.path.abspath(__file__)
)
LEARNING_STORE = os.environ.get("HERMES_LEARNING_STORE") or os.path.join(
    _STATE_DIR, "learning_store.json"
)
REFLECTION_INTERVAL = int(os.environ.get("REFLECTION_INTERVAL_SECONDS", "21600"))
REFLECTION_MODEL = os.environ.get("REFLECTION_MODEL", "") or os.environ.get(
    "HERMES_INFERENCE_MODEL", ""
)
REFLECTION_API_BASE = os.environ.get("REFLECTION_API_BASE", "") or os.environ.get(
    "OPENAI_BASE_URL", ""
)
REFLECTION_API_KEY = os.environ.get("REFLECTION_API_KEY", "") or os.environ.get(
    "OPENAI_API_KEY", ""
)

_QUALITY_LOG = []
_reflection_thread_started = False


class LearningStore:
    def __init__(self, path=None):
        self.path = path or LEARNING_STORE
        self.data = {
            "version": 1,
            "last_reflection_ts": 0,
            "reflection_count": 0,
            "session_count": 0,
            "common_questions": [],
            "user_tips": [],
            "response_patterns": [],
            "knowledge_items": [],
            "quality_history": [],
            "recent_quality_scores": [],
        }
        self._load()

    def _load(self):
        try:
            with open(self.path, "r", encoding="utf-8") as f:
                loaded = json.load(f)
                if isinstance(loaded, dict):
                    for k in self.data:
                        if k in loaded:
                            self.data[k] = loaded[k]
        except (FileNotFoundError, json.JSONDecodeError):
            pass

    def save(self):
        try:
            os.makedirs(os.path.dirname(os.path.abspath(self.path)), exist_ok=True)
            temp_path = self.path + ".tmp"
            with open(temp_path, "w", encoding="utf-8") as f:
                json.dump(self.data, f, ensure_ascii=False, indent=2)
            os.replace(temp_path, self.path)
        except Exception as e:
            print(f"[Learning] save failed: {e}", flush=True)

    def record_session(self):
        self.data["session_count"] += 1
        if self.data["session_count"] % 50 == 0:
            self.save()

    def add_knowledge(self, item: str):
        if item not in self.data["knowledge_items"]:
            self.data["knowledge_items"].append(item)
            if len(self.data["knowledge_items"]) > 50:
                self.data["knowledge_items"] = self.data["knowledge_items"][-50:]
            self.save()

    def add_pattern(self, pattern: str):
        if pattern not in self.data["response_patterns"]:
            self.data["response_patterns"].append(pattern)
            if len(self.data["response_patterns"]) > 30:
                self.data["response_patterns"] = self.data["response_patterns"][-30:]
            self.save()

    def add_tip(self, tip: str):
        if tip not in self.data["user_tips"]:
            self.data["user_tips"].append(tip)
            if len(self.data["user_tips"]) > 20:
                self.data["user_tips"] = self.data["user_tips"][-20:]
            self.save()

    def learn_question(self, question: str):
        q = question.strip().lower()[:80]
        if len(q) < 4:
            return
        for item in self.data["common_questions"]:
            if item["q"] == q:
                item["freq"] = item.get("freq", 1) + 1
                item["last_seen"] = int(time.time())
                self.save()
                return
        self.data["common_questions"].append(
            {"q": q, "freq": 1, "last_seen": int(time.time())}
        )
        self.data["common_questions"].sort(key=lambda x: x.get("freq", 0), reverse=True)
        if len(self.data["common_questions"]) > 20:
            self.data["common_questions"] = self.data["common_questions"][:20]
        self.save()

    def reflection_done(self):
        self.data["last_reflection_ts"] = int(time.time())
        self.data["reflection_count"] += 1
        self.save()

    def build_knowledge_context(self) -> str:
        parts = []
        if self.data["common_questions"]:
            top = self.data["common_questions"][:5]
            qs = "; ".join(f"{x['q']}({x['freq']}次)" for x in top)
            parts.append(f"[高频问题] {qs}")
        if self.data["knowledge_items"]:
            parts.append("[已学知识] " + " | ".join(self.data["knowledge_items"][-5:]))
        if self.data["user_tips"]:
            parts.append("[用户提示] " + "; ".join(self.data["user_tips"][-3:]))
        if self.data["response_patterns"]:
            parts.append("[经验模式] " + "; ".join(self.data["response_patterns"][-3:]))
        return "\n".join(parts)


_store: LearningStore | None = None
_API_BASE = ""
_API_KEY = ""
_MODEL = ""


def init(api_base="", api_key="", model=""):
    global _store, _API_BASE, _API_KEY, _MODEL
    _API_BASE = (api_base or REFLECTION_API_BASE).rstrip("/")
    _API_KEY = api_key or REFLECTION_API_KEY
    _MODEL = model or REFLECTION_MODEL
    _store = LearningStore()
    return _store


def get_store() -> LearningStore:
    global _store
    if _store is None:
        _store = LearningStore()
    return _store


# ---- 隐式质量评分 ----
def _score_conversation(user_text: str, reply: str, follow_up: int = 0) -> float:
    score = 3.0
    if not reply or len(reply.strip()) < 5:
        score -= 1.0
    for s in ("抱歉，", "无法处理", "不能执行", "需要管理员", "暂不支持"):
        if s in reply:
            score -= 0.3
            break
    if len(user_text.strip()) > 30:
        score += 0.3
    if len(user_text.strip()) > 80:
        score += 0.2
    if follow_up >= 3:
        score += 0.5
    if follow_up >= 8:
        score += 0.3
    if "?" in reply or "？" in reply:
        score += 0.2
    if any(c.isdigit() for c in reply[:50]) or "http" in reply:
        score += 0.3
    if user_text and len(user_text.strip()) < 5 and len(reply or "") > 50:
        score -= 0.2
    return max(0, min(5, score))


def record_conversation(
    user_text: str, reply: str, source: str = "", follow_up: int = 0
):
    store = get_store()
    score = _score_conversation(user_text, reply, follow_up)
    _QUALITY_LOG.append(
        {"ts": int(time.time()), "score": round(score, 2), "source": source}
    )
    if len(_QUALITY_LOG) > 200:
        _QUALITY_LOG.pop(0)
    store.data["recent_quality_scores"].append(score)
    if len(store.data["recent_quality_scores"]) > 200:
        store.data["recent_quality_scores"] = store.data["recent_quality_scores"][-200:]
    if user_text and len(user_text) > 8:
        store.learn_question(user_text[:120])
    store.record_session()


def get_quality_summary() -> dict:
    recent = _QUALITY_LOG[-50:] if _QUALITY_LOG else []
    if not recent:
        return {"count": 0, "avg_score": 0, "trend": "stable"}
    scores = [e["score"] for e in recent]
    avg = sum(scores) / len(scores)
    trend = "stable"
    if len(scores) >= 10:
        ra = sum(scores[-5:]) / 5
        oa = sum(scores[:5]) / 5
        trend = (
            "improving"
            if ra > oa + 0.3
            else ("declining" if ra < oa - 0.3 else "stable")
        )
    return {"count": len(recent), "avg_score": round(avg, 2), "trend": trend}


# ---- 复盘引擎 ----
def _run_periodic_reflection():
    global _reflection_thread_started
    if _reflection_thread_started:
        return
    _reflection_thread_started = True

    def _loop():
        while True:
            try:
                _do_reflection()
            except Exception as e:
                print(f"[Reflection] error: {e}", flush=True)
                traceback.print_exc()
            time.sleep(REFLECTION_INTERVAL)

    t = threading.Thread(target=_loop, daemon=True)
    t.start()
    print(f"[Reflection] scheduled every {REFLECTION_INTERVAL}s", flush=True)


def _do_reflection():
    store = get_store()
    local = _quality_local_stats()
    llm_result = {}
    if _API_BASE and _API_KEY and _MODEL:
        try:
            dialogues = _fetch_recent()
            if dialogues:
                llm_result = _reflect_llm(dialogues)
        except Exception as e:
            print(f"[Reflection] llm error: {e}", flush=True)
    _merge_reflection(store, llm_result, local)
    store.save()
    print(
        f"[Reflection] done: llm={bool(llm_result)} local={local.get('total', 0)}",
        flush=True,
    )


def _quality_local_stats() -> dict:
    recent = _QUALITY_LOG[-100:] if _QUALITY_LOG else []
    if not recent:
        return {}
    scores = [e["score"] for e in recent]
    high = sum(1 for s in scores if s >= 4)
    return {
        "total": len(scores),
        "avg": round(sum(scores) / len(scores), 2),
        "high": high,
    }


def _fetch_recent(count: int = 30) -> list:
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL") or "http://127.0.0.1:3000"
    ).rstrip("/")
    secret = (
        os.environ.get("CHATOPS_WEBHOOK_SECRET")
        or os.environ.get("NEWAPI_CHATOPS_SECRET")
        or ""
    )
    if not base or not secret:
        return []
    try:
        data = chatops_request(
            "/api/agent/chatops/history/get",
            {"limit": count},
            source=os.environ.get("BOT_PLATFORM") or "qq",
            timeout=8,
        )
        rows = data.get("data") or data.get("history") or []
        return rows if isinstance(rows, list) else []
    except Exception as e:
        print(f"[Reflection] fetch: {e}", flush=True)
        return []


def _reflect_llm(dialogues: list) -> dict:
    sp = "你是一个对话质量分析师。分析对话记录，输出复盘JSON。"
    prompt = json.dumps(
        {
            "task": "analyze_conversation_quality",
            "samples": dialogues[:15],
            "output": {
                "common_questions": ["高频问题列表"],
                "quality": "good|fair|poor",
                "improvements": ["改进建议"],
                "knowledge_items": ["学到的知识点"],
                "response_patterns": ["有效回复模式"],
            },
        },
        ensure_ascii=False,
    )[:4000]
    body = {
        "model": _MODEL,
        "messages": [
            {"role": "system", "content": sp},
            {"role": "user", "content": prompt},
        ],
        "temperature": 0.1,
        "max_tokens": 1000,
    }
    try:
        req = urllib.request.Request(
            _API_BASE + "/chat/completions",
            json.dumps(body).encode("utf-8"),
            headers={
                "Authorization": "Bearer " + _API_KEY,
                "Content-Type": "application/json",
            },
        )
        with urllib.request.urlopen(req, timeout=60) as resp:
            result = json.loads(resp.read().decode("utf-8"))
        content = result.get("choices", [{}])[0].get("message", {}).get("content", "")
        for tag in ("```json", "```"):
            if tag in content:
                content = content.split(tag)[1]
        content = content.strip().strip("`").strip()
        return json.loads(content) if content.startswith("{") else {}
    except Exception as e:
        print(f"[Reflection] llm call: {e}", flush=True)
        return {}


def _merge_reflection(store: LearningStore, llm: dict, local: dict):
    if llm.get("common_questions"):
        for q in llm["common_questions"][:3]:
            if isinstance(q, str) and len(q) > 4:
                store.learn_question(q)
    if llm.get("knowledge_items"):
        for k in llm["knowledge_items"][:3]:
            store.add_knowledge(k)
    if llm.get("response_patterns"):
        for p in llm["response_patterns"][:2]:
            store.add_pattern(p)
    if llm.get("improvements"):
        for tip in llm["improvements"][:2]:
            store.add_tip(tip)
    if local:
        store.data["quality_history"].append(
            {
                "ts": int(time.time()),
                "avg_score": local.get("avg", 0),
                "count": local.get("total", 0),
                "good_rate": round(
                    local.get("high", 0) / max(local.get("total", 1), 1) * 100, 1
                ),
            }
        )
        if len(store.data["quality_history"]) > 100:
            store.data["quality_history"] = store.data["quality_history"][-100:]
    store.reflection_done()


def build_dynamic_context() -> str:
    store = get_store()
    parts = []
    ctx = store.build_knowledge_context()
    if ctx:
        parts.append(ctx)
    qs = get_quality_summary()
    if qs.get("count", 0) > 0:
        parts.append(
            f"[服务统计] 近期{qs['count']}次对话均分{qs['avg_score']} 趋势:{qs['trend']}"
        )
    return "\n".join(parts)


def start_background_reflection():
    _run_periodic_reflection()
