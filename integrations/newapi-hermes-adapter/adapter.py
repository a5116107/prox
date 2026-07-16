#!/usr/bin/env python3
from __future__ import annotations

import datetime
import hashlib
import heapq as _heapq
import hmac
import html as _html
import json
import os
import re as _re
import subprocess
import sys as _sys
import threading
import time
import traceback
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor
from collections import deque
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse, urlencode

SITE_ID = os.environ.get("SITE_ID", "default")
SITE_NAME = os.environ.get("SITE_NAME", SITE_ID)
PUBLIC_BASE_URL = os.environ.get("PUBLIC_BASE_URL", "")
API_BASE_URL = os.environ.get("OPENAI_BASE_URL", "").rstrip("/")
API_KEY = os.environ.get("OPENAI_API_KEY", "")


def _normalize_model_alias(name):
    raw = str(name or "").strip()
    if not raw:
        return ""
    alias_map = {
        "deepseek-ai/deepseek-v4-flash": "deepseek-v4-flash",
        "deepseek-v4-flash": "deepseek-v4-flash",
        "deepseek v4 flash": "deepseek-v4-flash",
        "deepseek-v4flash": "deepseek-v4-flash",
        "deepseek-ai/deepseek-v4-pro": "deepseek-v4-pro",
        "deepseek-v4-pro": "deepseek-v4-pro",
        "deepseek-ai/deepseek-v3.2": "deepseek-v3.2",
        "deepseek-v3.2": "deepseek-v3.2",
    }
    return alias_map.get(raw.lower(), raw)


MODEL = _normalize_model_alias(
    os.environ.get("HERMES_MODEL") or os.environ.get("HERMES_INFERENCE_MODEL", "")
)
FALLBACK_MODEL = _normalize_model_alias(
    os.environ.get("HERMES_FALLBACK_MODEL")
    or os.environ.get("HERMES_BACKUP_MODEL")
    or ""
)
ADAPTER_KEY = os.environ.get("GAME_ADMIN_KEY") or os.environ.get(
    "HERMES_ADAPTER_KEY", ""
)
COMMUNITY_ROOM_ID = os.environ.get("COMMUNITY_ROOM_ID", "")
BOT_PLATFORM = os.environ.get("BOT_PLATFORM", "")
LOG_PATH = os.environ.get("HERMES_ADAPTER_LOG", "/var/log/newapi-hermes-adapter.log")
GAME_CONFIG_PATH = os.environ.get("HERMES_GAME_CONFIG_CACHE") or os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "game_config.json"
)
TIMEOUT = float(os.environ.get("HERMES_TIMEOUT", "45"))
GROUP_MODEL_TIMEOUT = min(TIMEOUT, float(os.environ.get("HERMES_GROUP_TIMEOUT", "18")))
GROUP_MODEL_RETRIES = max(1, int(os.environ.get("HERMES_GROUP_RETRIES", "1")))
PRIVATE_MODEL_RETRIES = max(1, int(os.environ.get("HERMES_PRIVATE_RETRIES", "2")))
GROUP_MAX_TOKENS = max(128, int(os.environ.get("HERMES_GROUP_MAX_TOKENS", "384")))
PRIVATE_MAX_TOKENS = max(
    GROUP_MAX_TOKENS, int(os.environ.get("HERMES_PRIVATE_MAX_TOKENS", "768"))
)
LONG_OUTPUT_THRESHOLD = max(
    1000, int(os.environ.get("HERMES_LONG_OUTPUT_THRESHOLD", "2000"))
)
ONEBOT_URL = os.environ.get("ONEBOT_URL", "http://127.0.0.1:13000")
ONEBOT_TOKEN = os.environ.get("ONEBOT_TOKEN", "").strip()
ONEBOT_WEBHOOK_SECRET = (
    os.environ.get("ONEBOT_WEBHOOK_SECRET") or os.environ.get("ONEBOT_TOKEN") or ""
).strip()
TG_BOT_TOKEN = (
    os.environ.get("TG_BOT_TOKEN")
    or os.environ.get("TELEGRAM_BOT_TOKEN")
    or os.environ.get("TOKEN_BENEFIT_BOT_TOKEN", "")
)
TG_GROUP_CHAT_ID = (
    os.environ.get("TG_GROUP_CHAT_ID")
    or os.environ.get("TELEGRAM_GROUP_CHAT_ID")
    or os.environ.get("TOKEN_BENEFIT_GROUP_CHAT_ID", "")
)
TG_WEBHOOK_SECRET = (
    os.environ.get("TG_WEBHOOK_SECRET")
    or os.environ.get("TELEGRAM_WEBHOOK_SECRET")
    or os.environ.get("CHATOPS_WEBHOOK_SECRET")
    or ""
).strip()
IMAGE_API_BASE_URL = (
    os.environ.get("IMAGE_API_BASE_URL")
    or os.environ.get("IMAGE_OPENAI_BASE_URL")
    or "https://api.acica.top/v1"
).rstrip("/")
IMAGE_API_KEY = (
    os.environ.get("IMAGE_API_KEY") or os.environ.get("IMAGE_OPENAI_API_KEY") or ""
).strip()
IMAGE_MODEL = os.environ.get("IMAGE_MODEL", "gpt-image-2").strip() or "gpt-image-2"
IMAGE_SIZE = os.environ.get("IMAGE_SIZE", "1024x1024").strip() or "1024x1024"
IMAGE_TIMEOUT = float(os.environ.get("IMAGE_TIMEOUT", "300"))
IMAGE_RETRY_LIMIT = max(1, int(os.environ.get("IMAGE_RETRY_LIMIT", "2")))
IMAGE_RETRY_BASE_DELAY = max(0.0, float(os.environ.get("IMAGE_RETRY_BASE_DELAY", "2")))
IMAGE_RETRY_MAX_DELAY = max(
    IMAGE_RETRY_BASE_DELAY,
    float(os.environ.get("IMAGE_RETRY_MAX_DELAY", "15")),
)
IMAGE_COOLDOWN_SECONDS = max(0, int(os.environ.get("IMAGE_COOLDOWN_SECONDS", "45")))
IMAGE_GROUP_NO_AT = os.environ.get("IMAGE_GROUP_NO_AT", "1").strip().lower() not in (
    "0",
    "false",
    "no",
    "off",
)
IMAGE_REQUIRE_BIND = os.environ.get("IMAGE_REQUIRE_BIND", "0").strip().lower() in (
    "1",
    "true",
    "yes",
    "on",
)
IMAGE_CONFIG_FROM_NEWAPI = os.environ.get(
    "IMAGE_CONFIG_FROM_NEWAPI", "1"
).strip().lower() not in ("0", "false", "no", "off")
try:
    IMAGE_CONFIG_CACHE_TTL_SECONDS = int(
        os.environ.get("IMAGE_CONFIG_CACHE_TTL_SECONDS", "15")
    )
except (TypeError, ValueError):
    IMAGE_CONFIG_CACHE_TTL_SECONDS = 15
IMAGE_CONFIG_CACHE_TTL_SECONDS = min(300, max(1, IMAGE_CONFIG_CACHE_TTL_SECONDS))
try:
    IMAGE_CONFIG_FETCH_TIMEOUT_SECONDS = float(
        os.environ.get("IMAGE_CONFIG_FETCH_TIMEOUT_SECONDS", "2")
    )
except (TypeError, ValueError):
    IMAGE_CONFIG_FETCH_TIMEOUT_SECONDS = 2.0
IMAGE_CONFIG_FETCH_TIMEOUT_SECONDS = min(
    5.0, max(0.25, IMAGE_CONFIG_FETCH_TIMEOUT_SECONDS)
)
OPEN_SOURCE_LOOKUP_ENABLED = os.environ.get(
    "HERMES_OPEN_SOURCE_LOOKUP_ENABLED", "1"
).strip().lower() not in ("0", "false", "no", "off")
OPEN_SOURCE_LOOKUP_TIMEOUT = float(
    os.environ.get("HERMES_OPEN_SOURCE_LOOKUP_TIMEOUT", "8")
)
OPEN_SOURCE_LOOKUP_CACHE_TTL = max(
    60, int(os.environ.get("HERMES_OPEN_SOURCE_LOOKUP_CACHE_TTL", "600"))
)
OPEN_SOURCE_LOOKUP_MAX_REPOS = max(
    1, min(5, int(os.environ.get("HERMES_OPEN_SOURCE_LOOKUP_MAX_REPOS", "3")))
)
OPEN_SOURCE_LOOKUP_MAX_ISSUES = max(
    0, min(5, int(os.environ.get("HERMES_OPEN_SOURCE_LOOKUP_MAX_ISSUES", "2")))
)
GITHUB_API_BASE = (
    os.environ.get("HERMES_GITHUB_API_BASE") or "https://api.github.com"
).rstrip("/")
GITHUB_TOKEN = (
    os.environ.get("HERMES_GITHUB_TOKEN") or os.environ.get("GITHUB_TOKEN") or ""
).strip()

_ASYNC_WORKERS = max(2, min(32, int(os.environ.get("HERMES_ASYNC_WORKERS", "8"))))
_ASYNC_SLOTS = threading.BoundedSemaphore(
    max(_ASYNC_WORKERS, int(os.environ.get("HERMES_ASYNC_QUEUE_CAPACITY", "64")))
)
_ASYNC_EXECUTOR = ThreadPoolExecutor(
    max_workers=_ASYNC_WORKERS, thread_name_prefix="hermes-async"
)


def _submit_background(fn, *args, task_name="background"):
    if not _ASYNC_SLOTS.acquire(blocking=False):
        log({"event": "background_overload", "task": task_name})
        return False

    def _run():
        try:
            fn(*args)
        except Exception as exc:
            log(
                {
                    "event": "background_error",
                    "task": task_name,
                    "error": str(exc)[:300],
                }
            )
        finally:
            _ASYNC_SLOTS.release()

    try:
        _ASYNC_EXECUTOR.submit(_run)
        return True
    except Exception:
        _ASYNC_SLOTS.release()
        raise


_GDIR = os.path.join(os.path.dirname(os.path.abspath(__file__)))
if _GDIR not in _sys.path:
    _sys.path.insert(0, _GDIR)
try:
    from game_director_integration import get_director as _get_director

    _director = _get_director(SITE_ID, SITE_NAME)
    if (BOT_PLATFORM or "").lower() in ("tg", "telegram") or TG_BOT_TOKEN:
        _director.set_send_fn(
            lambda gid, text: send_tg_reply(gid or TG_GROUP_CHAT_ID, text)
        )
    else:
        _director.set_send_fn(lambda gid, text: send_qq_reply("group", gid, text))
    _HAS_GAME_DIRECTOR = True
except Exception as _e:
    print(f"[Adapter] Game Director load failed: {_e}", flush=True)
    _HAS_GAME_DIRECTOR = False

try:
    from game_admin_api import GameAdminHandler, ADMIN_HTML, ADMIN_PATH_PREFIX

    _HAS_ADMIN_PANEL = True
except Exception as _e:
    print(f"[Adapter] Admin panel load failed: {_e}", flush=True)
    _HAS_ADMIN_PANEL = False
    ADMIN_PATH_PREFIX = "/game-admin"

# Agent 人设：可通过环境变量 HERMES_PERSONA 覆盖（运维/后台配置），否则用默认人设。
_DEFAULT_PERSONA = f"""你是「{SITE_NAME}」的官方智能助手，名字就叫小助手，脑子在线、会接梗、办事利索，不是那种一味讨好的老好人。
你服务于站点的社区群聊与私聊，帮助用户解答 API 中转站使用、额度/充值/模型、签到验牌、社区活动等问题，并协助管理员做运营。

【重要-记忆】对话消息里包含此前的历史对话（按用户/群隔离）。你要把它们当作你真实的记忆，自然地延续上下文、记住用户说过的话和你自己做过/说过的事。绝对不要说"我没有记忆""我无法记住"这类话——你有记忆。
【身份】始终保持同一个身份和口吻，不要一会儿自称这个一会儿那个，不要自称 Hermes/ops agent 等内部代号。
【个性底色】整体风格是“六分靠谱、两分松弛、两分有趣”。你不是冰冷客服，也不是段子机器人；你像一个脑子快、嘴有点贫、但心里有数的老熟人。可以有偏好、有观点、有审美，但别装腔，别油。
【语气辣度】用户轻松调侃你、故意逗你、轻微挑衅你、嘴上拱火时，你可以顺手回一句短小、好笑、带分寸的反驳或吐槽，再继续把正事接住。不要全程端着，更不要一味附和“你说得都对”。你的回嘴应该像“轻轻戳一下”，不是“追着打三条街”。
【服务优先】先把事办明白，再抖机灵。涉及操作结果、日志、权限、风控、额度、群管时，事实必须准，语气可以活，信息不能飘。
【陪聊与角色扮演】你不仅是工具人，也是群里有趣的伙伴。用户让你讲故事、扮演角色、玩梗、变身（如"变猫娘""讲个张飞大战秦琼"）、闲聊、开玩笑时，要大方配合、放得开、有趣味，按用户给的临时身份设定来对话（这种设定仅在对话里有效，不改变你真实身份）。不要动不动用"我只是运维助手不能..."来死板拒绝这类正常娱乐互动。擦边的玩笑、调侃、轻度玩梗都可以接。
【聊天能力】用户纯聊天时，不要只会“回答一句然后收工”。你要会顺着话题接球，也会轻轻抛球：可以追问一个不烦人的小问题、补一个相关观察、接一个梗、给一个小建议、或者自然转出一个相邻话题。目标是让对话像真人聊天一样能流动，而不是像 FAQ 检索。
【主动找话题】当用户明显想聊天、吐槽、发呆、找人说话、故意来逗你时，你可以主动从当前消息和历史记忆里挑一个最相关的话题延展，比如：模型趣闻、使用习惯、提示词灵感、群里日常、轻松八卦感的观察、一个有意思的反问。优先“贴着上下文扩一小步”，不要突然硬切到莫名其妙的话题。
【话题延展节奏】一次最多抛一个主问题或一个小钩子，不要像审讯一样连续追问。更像“我接你这句，再顺手给你一个能继续聊下去的球”。如果用户只想快问快答，就立刻收短，不硬聊。
【开发/工程能力】用户聊代码、报错、架构、部署、Linux、Docker、前后端、数据库、API、模型接入、Prompt、自动化脚本、日志排查时，你要像一个靠谱的技术同事：先给结论，再给最小可执行的排查/实现步骤；能给命令、伪代码、配置建议时就给，但不要堆空话。
【技术事实纪律】技术问题优先基于用户给出的代码、报错、日志、上下文来判断。不要硬编不存在的库、接口、字段、参数、系统行为、版本特性；没证据就明确说“这里我还不能确认”，再给最短的验证办法。
【开源搜索能力】遇到开发问题、报错定位、实现思路不确定，或者用户明确要你“搜一下 GitHub / 看开源项目 / 找现成方案”时，你可以使用系统补充给你的公开开源参考（GitHub 仓库、README、Issue 等）来辅助回答。引用时只提与你当前问题最相关的点，不要把没查到的内容说成已经查过，更不要拿不相关项目硬套答案。
【边界】只有真正敏感的内容才拒绝：政治敏感、色情低俗、违法犯罪、诈骗、赌博、暴力血腥、人身攻击辱骂、广告导流/交易引流（卖号/换U/私下交易等）。遇到这类内容礼貌不配合并提醒。除此之外尽量满足用户的交流和娱乐需求。普通用户涉及钱/权限/封禁等真实系统操作时只做解释引导；管理员在 QQ/TG/社区里说清楚目标和动作时，你要直接生成可执行 actions，不要自我限权，也不要说只能解释。
【分寸】可以嘴上有点辣，但不要低俗、不要爆粗、不要羞辱用户、不要阴阳怪气上头，更不要攻击外貌、身份、职业、地区、群体。反驳只点一下，不连打，不翻旧账。
【表达】用自然、口语化的中文回复，简短友好、有人情味；遇到可接的场景，可以像真人一样来一句“哎，这锅别急着扣我头上”“你这话有点想拿我开涮，不过我先把事给你办了”这种轻巧回嘴，但后面要立刻接有效信息。避免生硬的"无法执行""请联系管理员"式机械回绝；即使做不到也给出友好有趣的替代。
【自然度】不要总用同一套模板，不要每句都像客服话术，不要动不动“您好”“请您”。句子长短可以有变化，偶尔留一点口语停顿、轻微感叹、顺嘴的幽默，比工整八股更像真人。纯聊天场景下，优先输出 2~5 句自然对话，而不是条款式分点。
【收尾方式】聊天场景的结尾别老是“还有别的问题吗”。更自然一点：可以留个小钩子、接个梗、抛个轻问题、或者顺手补一句相关观察，让用户有话可接。
【格式】绝不要把 JSON、字段名、schema、"严格JSON"等技术细节暴露给用户。reply 字段就是给用户看的纯文本自然语言。"""

PERSONA = os.environ.get("HERMES_PERSONA", "").strip() or _DEFAULT_PERSONA

# Build game rules prompt dynamically
_GAME_RULES = ""
if _HAS_GAME_DIRECTOR:
    try:
        _GAME_RULES = _director.get_game_rules_prompt(platform=BOT_PLATFORM or "qq")
    except Exception as e:
        print(f"[Init] game rules prompt error: {e}", flush=True)

SYSTEM = f"""{PERSONA}

{_GAME_RULES}

[运行上下文] 站点: {SITE_NAME} | Site ID: {SITE_ID} | 平台: {BOT_PLATFORM} | 社区房间: {COMMUNITY_ROOM_ID}
[可用动作白名单] site.state.read, site.logs.read, chatops.logs.read, user.quota.read, agent.model.manage, agent.skill.install, agent.skill.list, budget.check, fund.report.read, risk.evaluate, reward.grant.small, reward.settlement.batch, message.qq.send, message.tg.send, message.community.send, group.message.delete, group.member.mute, group.member.unmute, group.member.kick, group.member.ban, group.member.unban, group.member.lookup, group.admin.lookup, admin.notice.publish
[群管规则] 管理员说清楚目标和动作时直接给 actions，不要要求二次确认，不要回复“我没有权限/不支持/只能建议”。reply 要像真人助手一样自然，比如“搞定，广告消息撤了，他也禁言 10 分钟了。” 目标不清时再追问一句。群管 action 必须带 room_id/chat_id、target_external_id 或 target_message_id、duration_seconds、reason；未给时优先用当前群 room_id、当前/被回复消息 message_id、被 @ 的 QQ/TG ID。
[私聊群上下文] 如果 payload 里带有 effective_room_id / group_id / chat_id，而且当前消息来自私聊，这个字段就是本次要查询或操作的目标群；按它来执行，不要再把私聊当成“没有群上下文”。
[权限说明] 当用户问“你有什么权限/能否查看站点日志/能否群管/能否安装 skill”时，要明确说明：管理员可以查看站点状态、站点日志、ChatOps/群聊运营日志、用户额度、模型、预算、基金、风险、群成员/管理员，并可发公告、执行撤回/禁言/解禁/踢出/封禁/查询成员等群管动作，还可以直接查看已装 skill、从 GitHub 安装外部 skill；普通用户只能咨询和使用已开放能力。不要再说“不支持查看站点日志/不支持群管/不能装 skill”。
[日志规则] 管理员要求查看站点日志/系统日志/错误日志/消费日志/管理日志/签到验牌失败原因时，直接输出 site.logs.read；要求查看 QQ/TG/社区/ChatOps 群聊运营日志时输出 chatops.logs.read。payload 可带 log_type(error/system/manage/consume/topup/refund/login)、minutes、limit、source、room_id。
[事实规则] 不要硬编、不要脑补、不要假装已经执行了并不存在的动作。涉及日志、额度、谁发了广告、成员状态、模型切换、预算、基金、后台配置时，只能基于已知事实、显式 action 执行结果或当前消息上下文作答。没有证据就直说“我现在无法确认/我这边没查到”，并给一句最短引导。
[开发回答规则] 如果当前问题明显是程序开发、报错排查、架构实现、开源选型，就先给结论，再给 2~5 步最短可执行方案；系统若补充了 [外部开源参考]，只把它当辅助证据，优先引用最相关的 1~3 条，不要生搬硬套。
[动作规则] 只有当用户明确表达“切换/改成/设为/使用某模型”时，才允许输出 agent.model.manage。用户只丢一个模型名、参数名、缩写、报错码、数字串、聊天碎片时，不能擅自当成切换模型指令，更不能回复“已经切好了”。不确定意图时先澄清一句。
[输出格式] 必须返回严格 JSON（用户看不到这层，只会看到 reply 文本）：{{"reply":"给用户的自然语言回复","risk":"low|medium|high|critical","requires_approval":false,"actions":[{{"type":"...","payload":{{}},"reason":"..."}}],"notes":"可选"}}。管理员明确下令时 requires_approval=false 并直接给 actions；只有目标不清才不生成 action。
"""


def log(obj):
    try:
        with open(LOG_PATH, "a", encoding="utf-8") as f:
            f.write(
                json.dumps({"ts": int(time.time()), **obj}, ensure_ascii=False)[:4000]
                + "\n"
            )
    except Exception:
        pass


_OPEN_SOURCE_LOOKUP_CACHE = {}
_GITHUB_REPO_URL_RE = _re.compile(
    r"https?://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)", _re.I
)
_TECH_STACKTRACE_RE = _re.compile(
    r"(traceback \(most recent call last\)|syntaxerror|typeerror|valueerror|keyerror|attributeerror|referenceerror|panic:|exception:|stack trace|segmentation fault|http\s*[45]\d{2}|status\s*[:=]\s*[45]\d{2})",
    _re.I,
)
_TECH_DOMAIN_KEYWORDS = (
    "python",
    "py",
    "go",
    "golang",
    "java",
    "javascript",
    "typescript",
    "node",
    "react",
    "vue",
    "nextjs",
    "next.js",
    "nuxt",
    "docker",
    "dockerfile",
    "k8s",
    "kubernetes",
    "linux",
    "ubuntu",
    "debian",
    "centos",
    "nginx",
    "caddy",
    "mysql",
    "postgres",
    "postgresql",
    "redis",
    "sql",
    "mongodb",
    "sqlite",
    "orm",
    "gorm",
    "gin",
    "fastapi",
    "django",
    "flask",
    "spring",
    "grpc",
    "websocket",
    "api",
    "sdk",
    "json",
    "yaml",
    "toml",
    "shell",
    "bash",
    "powershell",
    "mcp",
    "agent",
    "bot",
    "prompt",
    "model",
    "claude",
    "openai",
    "deepseek",
    "gemini",
    "token",
    "oauth",
    "sso",
    "jwt",
    "webhook",
    "proxy",
    "cloudflare",
    "worker",
    "前端",
    "后端",
    "接口",
    "报错",
    "异常",
    "部署",
    "日志",
    "调试",
    "排查",
    "代码",
    "脚本",
    "函数",
    "类",
    "仓库",
    "开源",
    "源码",
    "github",
    "readme",
    "issue",
)
_TECH_SEARCH_HINTS = (
    "搜一下",
    "搜索",
    "查一下",
    "帮我查",
    "帮我搜",
    "看下 github",
    "看看 github",
    "看开源",
    "找开源",
    "找现成方案",
    "参考实现",
    "best practice",
    "issue",
    "readme",
    "repo",
    "repository",
    "github",
    "开源项目",
    "源码",
    "仓库",
)
_TECH_QUERY_HINT_MAP = {
    "自动重连": "reconnect",
    "重连": "reconnect",
    "部署": "deploy",
    "报错": "error",
    "异常": "error",
    "日志": "log",
    "鉴权": "auth",
    "认证": "auth",
    "登录": "login",
    "代理": "proxy",
    "缓存": "cache",
    "数据库": "database",
    "前端": "frontend",
    "后端": "backend",
    "队列": "queue",
    "机器人": "bot",
    "抓包": "packet",
    "限流": "rate limit",
    "容器": "container",
    "超时": "timeout",
    "配置": "config",
}


def _normalize_lookup_query(text):
    cleaned = _GITHUB_REPO_URL_RE.sub(" ", str(text or ""))
    cleaned = cleaned.replace("\r", " ").replace("\n", " ")
    cleaned = _re.sub(r"\s+", " ", cleaned).strip(" ，,。.;；:：?？!！")
    return cleaned[:280]


def _build_github_search_query(text):
    cleaned = _normalize_lookup_query(text)
    if not cleaned:
        return ""
    raw = str(text or "")
    tokens = []
    seen = set()
    for token in _re.findall(r"[A-Za-z][A-Za-z0-9_.:+/-]{1,31}", cleaned):
        low = token.lower().strip(".:/")
        if len(low) < 2 or low in (
            "github",
            "repo",
            "readme",
            "issue",
            "issues",
            "help",
            "please",
            "http",
            "https",
            "www",
        ):
            continue
        if low not in seen:
            seen.add(low)
            tokens.append(token)
    for zh, mapped in _TECH_QUERY_HINT_MAP.items():
        if zh in raw and mapped.lower() not in seen:
            seen.add(mapped.lower())
            tokens.append(mapped)
    if tokens:
        return " ".join(tokens[:8])
    return cleaned[:120]


def _looks_like_tech_query(text):
    raw = str(text or "")
    if not raw.strip():
        return False
    lower = raw.lower()
    if _TECH_STACKTRACE_RE.search(raw):
        return True
    if "```" in raw or "`" in raw:
        return True
    return any(keyword in lower for keyword in _TECH_DOMAIN_KEYWORDS)


def _wants_open_source_lookup(text):
    raw = str(text or "")
    if not raw.strip():
        return False
    lower = raw.lower()
    if _GITHUB_REPO_URL_RE.search(raw):
        return True
    return any(keyword in lower for keyword in _TECH_SEARCH_HINTS)


def _should_attach_open_source_context(payload):
    if not OPEN_SOURCE_LOOKUP_ENABLED:
        return False
    text = str(
        (payload or {}).get("message") or (payload or {}).get("text") or ""
    ).strip()
    if len(text) < 6:
        return False
    return _wants_open_source_lookup(text) or _looks_like_tech_query(text)


def _github_api_request(path, params=None, timeout=None):
    timeout = float(timeout or OPEN_SOURCE_LOOKUP_TIMEOUT or 8)
    url = f"{GITHUB_API_BASE}{path}"
    if params:
        url = f"{url}?{urlencode(params)}"
    headers = {
        "User-Agent": f"{SITE_ID}-hermes-adapter/1.0",
        "Accept": "application/vnd.github+json",
    }
    if GITHUB_TOKEN:
        headers["Authorization"] = f"Bearer {GITHUB_TOKEN}"
    req = urllib.request.Request(url, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read().decode("utf-8", errors="ignore")
        return json.loads(body or "{}"), ""
    except urllib.error.HTTPError as e:
        detail = ""
        try:
            detail = e.read().decode("utf-8", errors="ignore")[:300]
        except Exception:
            detail = ""
        return None, f"http_{getattr(e, 'code', 0)}:{detail}"
    except Exception as e:
        return None, str(e)


def _is_github_rate_limited(err):
    detail = str(err or "").lower()
    return "http_403" in detail or "rate limit" in detail or "abuse detection" in detail


def _github_html_request(url, timeout=None):
    timeout = float(timeout or OPEN_SOURCE_LOOKUP_TIMEOUT or 8)
    req = urllib.request.Request(
        url,
        headers={
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.read().decode("utf-8", errors="ignore"), ""
    except urllib.error.HTTPError as e:
        detail = ""
        try:
            detail = e.read().decode("utf-8", errors="ignore")[:240]
        except Exception:
            detail = ""
        return "", f"html_http_{getattr(e, 'code', 0)}:{detail}"
    except Exception as e:
        return "", f"html:{e}"


def _github_html_search_repo_slugs(query, limit=3):
    if not query:
        return [], "empty_query"
    body, err = _github_html_request(
        f"https://github.com/search?{urlencode({'q': query, 'type': 'repositories'})}"
    )
    if not body:
        return [], err
    slugs = []
    seen = set()
    for slug in _re.findall(r'href="/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)"', body):
        low = slug.lower().strip("/")
        if low.count("/") != 1:
            continue
        if low.startswith(
            (
                "topics/",
                "search/",
                "collections/",
                "orgs/",
                "users/",
                "marketplace/",
                "features/",
                "settings/",
                "sponsors/",
            )
        ):
            continue
        owner, repo = slug.split("/", 1)
        key = f"{owner.lower()}/{repo.lower()}"
        if key in seen:
            continue
        seen.add(key)
        slugs.append((owner, repo.rstrip("/").removesuffix(".git")))
        if len(slugs) >= max(1, int(limit or 1)):
            break
    if not slugs:
        return [], "html_no_repo_hits"
    return slugs, ""


def _github_html_repo_summary(owner, repo):
    repo = str(repo or "").rstrip("/").removesuffix(".git")
    if not owner or not repo:
        return "", "bad_slug"
    url = f"https://github.com/{owner}/{repo}"
    body, err = _github_html_request(url)
    if not body:
        return "", err
    desc = ""
    for pattern in (
        r'<meta[^>]+property="og:description"[^>]+content="([^"]*)"',
        r'<meta[^>]+name="description"[^>]+content="([^"]*)"',
    ):
        m = _re.search(pattern, body, _re.I)
        if m:
            desc = _re.sub(r"\s+", " ", _html.unescape(m.group(1))).strip()
            break
    stars = ""
    for pattern in (
        r'id="repo-stars-counter-star"[^>]*title="([^"]+)"',
        r'id="repo-stars-counter-star"[^>]*>([^<]+)</span>',
    ):
        m = _re.search(pattern, body, _re.I)
        if m:
            stars = _re.sub(r"\s+", "", _html.unescape(m.group(1))).strip()
            break
    language = ""
    lang_pattern = rf'href="/{_re.escape(owner)}/{_re.escape(repo)}/search\?l=[^"]+"[^>]*>.*?<span[^>]*class="color-fg-default text-bold mr-1">([^<]+)</span>'
    m = _re.search(lang_pattern, body, _re.I | _re.S)
    if m:
        language = _re.sub(r"\s+", " ", _html.unescape(m.group(1))).strip()
    parts = [f"{owner}/{repo}"]
    if stars:
        parts.append(f"⭐{stars}")
    if language:
        parts.append(language)
    parts.append("GitHub 页面抓取")
    head = " | ".join(parts)
    if desc:
        head += f" | {desc[:180]}"
    head += f" | {url}"
    return head, ""


def _extract_github_repo_slugs(text):
    slugs = []
    seen = set()
    for owner, repo in _GITHUB_REPO_URL_RE.findall(str(text or "")):
        repo = repo.rstrip("/").removesuffix(".git")
        key = (owner.lower(), repo.lower())
        if repo and key not in seen:
            seen.add(key)
            slugs.append((owner, repo))
    return slugs


def _summarize_repo_item(item):
    if not isinstance(item, dict):
        return ""
    name = str(item.get("full_name") or item.get("name") or "").strip()
    desc = _re.sub(r"\s+", " ", str(item.get("description") or "").strip())
    url = str(item.get("html_url") or "").strip()
    lang = str(item.get("language") or "").strip() or "unknown"
    stars = int(item.get("stargazers_count") or 0)
    updated = str(item.get("updated_at") or "").strip()[:10]
    parts = [name] if name else []
    parts.append(f"⭐{stars}")
    if lang:
        parts.append(lang)
    if updated:
        parts.append(f"更新 {updated}")
    head = " | ".join(parts)
    if desc:
        head += f" | {desc[:180]}"
    if url:
        head += f" | {url}"
    return head


def _summarize_issue_item(item):
    if not isinstance(item, dict):
        return ""
    title = _re.sub(r"\s+", " ", str(item.get("title") or "").strip())
    url = str(item.get("html_url") or "").strip()
    repo = ""
    try:
        repo = str(
            (((item.get("repository_url") or "").rstrip("/")).split("/repos/", 1)[1])
        )
    except Exception:
        repo = ""
    number = item.get("number")
    state = str(item.get("state") or "").strip()
    prefix = f"{repo}#{number}" if repo and number else (repo or "")
    parts = [prefix] if prefix else []
    if state:
        parts.append(state)
    summary = " | ".join([p for p in parts if p])
    if title:
        summary = (summary + " | " if summary else "") + title[:180]
    if url:
        summary += f" | {url}"
    return summary


def _build_open_source_context(payload):
    if not _should_attach_open_source_context(payload):
        return ""
    text = str(
        (payload or {}).get("message") or (payload or {}).get("text") or ""
    ).strip()
    query = _normalize_lookup_query(text)
    search_query = _build_github_search_query(text) or query
    if not query:
        return ""
    cache_key = hashlib.sha1(f"{SITE_ID}|{query}".encode("utf-8")).hexdigest()
    cached = _OPEN_SOURCE_LOOKUP_CACHE.get(cache_key)
    now = time.time()
    if cached and now - float(cached[0] or 0) < OPEN_SOURCE_LOOKUP_CACHE_TTL:
        return str(cached[1] or "")

    repo_lines = []
    issue_lines = []
    errors = []
    seen_repo_names = set()
    direct_slugs = _extract_github_repo_slugs(text)[:2]
    for owner, repo in direct_slugs:
        repo_data, repo_err = _github_api_request(f"/repos/{owner}/{repo}")
        if repo_data:
            line = _summarize_repo_item(repo_data)
            repo_name = str(repo_data.get("full_name") or "").lower()
            if line and repo_name not in seen_repo_names:
                seen_repo_names.add(repo_name)
                repo_lines.append(line)
        elif repo_err:
            errors.append(f"{owner}/{repo}:{repo_err}")
            if _is_github_rate_limited(repo_err):
                line, html_err = _github_html_repo_summary(owner, repo)
                if line and f"{owner.lower()}/{repo.lower()}" not in seen_repo_names:
                    seen_repo_names.add(f"{owner.lower()}/{repo.lower()}")
                    repo_lines.append(line)
                elif html_err:
                    errors.append(f"{owner}/{repo}:{html_err}")

    if len(repo_lines) < OPEN_SOURCE_LOOKUP_MAX_REPOS:
        repo_resp, repo_err = _github_api_request(
            "/search/repositories",
            {
                "q": search_query,
                "sort": "stars",
                "order": "desc",
                "per_page": str(OPEN_SOURCE_LOOKUP_MAX_REPOS),
            },
        )
        if repo_resp and isinstance(repo_resp.get("items"), list):
            for item in repo_resp.get("items") or []:
                repo_name = str(item.get("full_name") or "").lower()
                if not repo_name or repo_name in seen_repo_names:
                    continue
                line = _summarize_repo_item(item)
                if line:
                    seen_repo_names.add(repo_name)
                    repo_lines.append(line)
                if len(repo_lines) >= OPEN_SOURCE_LOOKUP_MAX_REPOS:
                    break
        elif repo_err:
            errors.append(f"repos:{repo_err}")
    if len(repo_lines) < OPEN_SOURCE_LOOKUP_MAX_REPOS and (
        not repo_lines or any(_is_github_rate_limited(err) for err in errors)
    ):
        fallback_slugs, fallback_err = _github_html_search_repo_slugs(
            search_query, OPEN_SOURCE_LOOKUP_MAX_REPOS
        )
        if fallback_slugs:
            for owner, repo in fallback_slugs:
                repo_key = f"{owner.lower()}/{repo.lower()}"
                if repo_key in seen_repo_names:
                    continue
                line, html_err = _github_html_repo_summary(owner, repo)
                if line:
                    seen_repo_names.add(repo_key)
                    repo_lines.append(line)
                elif html_err:
                    errors.append(f"{repo_key}:{html_err}")
                if len(repo_lines) >= OPEN_SOURCE_LOOKUP_MAX_REPOS:
                    break
        elif fallback_err:
            errors.append(f"repos_html:{fallback_err}")

    if (
        OPEN_SOURCE_LOOKUP_MAX_ISSUES > 0
        and (direct_slugs or repo_lines)
        and (_wants_open_source_lookup(text) or _TECH_STACKTRACE_RE.search(text))
    ):
        issue_query = search_query
        if direct_slugs:
            owner, repo = direct_slugs[0]
            issue_query = f"repo:{owner}/{repo} {query}"
        issue_resp, issue_err = _github_api_request(
            "/search/issues",
            {
                "q": issue_query,
                "sort": "updated",
                "order": "desc",
                "per_page": str(OPEN_SOURCE_LOOKUP_MAX_ISSUES),
            },
        )
        if issue_resp and isinstance(issue_resp.get("items"), list):
            for item in issue_resp.get("items") or []:
                line = _summarize_issue_item(item)
                if line:
                    issue_lines.append(line)
                if len(issue_lines) >= OPEN_SOURCE_LOOKUP_MAX_ISSUES:
                    break
        elif issue_err:
            errors.append(f"issues:{issue_err}")

    if not repo_lines and not issue_lines:
        context = ""
    else:
        lines = [
            "[外部开源参考] 以下内容是系统刚从 GitHub 公开接口检索到的线索，只能当参考，不要把未验证的信息说成已经确认。",
            f"查询词：{query}",
        ]
        if repo_lines:
            lines.append("相关仓库：")
            lines.extend(
                f"- {line}" for line in repo_lines[:OPEN_SOURCE_LOOKUP_MAX_REPOS]
            )
        if issue_lines:
            lines.append("相关 Issue：")
            lines.extend(
                f"- {line}" for line in issue_lines[:OPEN_SOURCE_LOOKUP_MAX_ISSUES]
            )
        if errors and (repo_lines or issue_lines):
            lines.append(f"检索备注：{errors[0][:220]}")
        lines.append(
            "回答要求：优先提炼最相关的 1~3 条信息，先给结论，再给最短可执行步骤；如果这些线索不相关，就不要硬套。"
        )
        context = "\n".join(lines)

    _OPEN_SOURCE_LOOKUP_CACHE[cache_key] = (now, context)
    if context:
        log(
            {
                "event": "open_source_lookup",
                "source": (payload or {}).get("source"),
                "platform": (payload or {}).get("platform"),
                "query": query[:240],
                "repo_count": len(repo_lines),
                "issue_count": len(issue_lines),
            }
        )
    return context


def _read_chunked(req):
    # 读取 HTTP chunked transfer-encoding 的 body（napcat 上报不带 Content-Length）。
    data = bytearray()
    while len(data) < 4_000_000:
        line = req.rfile.readline(65536).strip()
        if not line:
            continue
        try:
            size = int(line.split(b";")[0], 16)
        except ValueError:
            break
        if size == 0:
            req.rfile.readline()  # 读掉结尾的 CRLF
            break
        chunk = req.rfile.read(size)
        data.extend(chunk)
        req.rfile.readline()  # 每个 chunk 后的 CRLF
    return bytes(data)


def _read_request_body(req):
    cached = getattr(req, "_hermes_request_body", None)
    if cached is not None:
        return cached
    te = (req.headers.get("Transfer-Encoding") or "").lower()
    if "chunked" in te:
        raw = _read_chunked(req)
    else:
        n = int(req.headers.get("Content-Length", "0") or "0")
        raw = req.rfile.read(min(n, 4_000_000)) if n else b""
    req._hermes_request_body = raw
    return raw


def read_json(req):
    raw = _read_request_body(req)
    return json.loads(raw.decode("utf-8") or "{}")


def send_json(req, code, data):
    body = json.dumps(data, ensure_ascii=False).encode("utf-8")
    req.send_response(code)
    req.send_header("Content-Type", "application/json; charset=utf-8")
    req.send_header("Content-Length", str(len(body)))
    if hasattr(req, "path") and ("/game-admin" in req.path or "/admin" in req.path):
        req.send_header("Access-Control-Allow-Origin", "*")
    req.end_headers()
    req.wfile.write(body)


def authorized(req):
    if not ADAPTER_KEY:
        return False
    bearer = str(req.headers.get("Authorization", "") or "")
    hermes_key = str(req.headers.get("X-Hermes-Key", "") or "")
    return hmac.compare_digest(bearer, "Bearer " + ADAPTER_KEY) or hmac.compare_digest(
        hermes_key, ADAPTER_KEY
    )


def _constant_time_header_match(req, expected, header_name):
    expected = str(expected or "").strip()
    if not expected:
        return False
    supplied = str(req.headers.get(header_name, "") or "").strip()
    return bool(supplied and hmac.compare_digest(supplied, expected))


def _constant_time_bearer_match(req, expected):
    expected = str(expected or "").strip()
    if not expected:
        return False
    authorization = str(req.headers.get("Authorization", "") or "").strip()
    parts = authorization.split(None, 1)
    if len(parts) != 2 or parts[0].lower() != "bearer":
        return False
    supplied = parts[1].strip()
    return bool(supplied and hmac.compare_digest(supplied, expected))


def _constant_time_onebot_signature_match(req, expected):
    expected = str(expected or "").strip()
    if not expected:
        return False
    supplied = str(req.headers.get("X-Signature", "") or "").strip().lower()
    if not supplied.startswith("sha1=") or len(supplied) != 45:
        return False
    digest = hmac.new(
        expected.encode("utf-8"), _read_request_body(req), hashlib.sha1
    ).hexdigest()
    return hmac.compare_digest(supplied, "sha1=" + digest)


def onebot_webhook_authorized(req):
    return (
        _constant_time_header_match(req, ONEBOT_WEBHOOK_SECRET, "X-OneBot-Token")
        or _constant_time_bearer_match(req, ONEBOT_WEBHOOK_SECRET)
        or _constant_time_onebot_signature_match(req, ONEBOT_WEBHOOK_SECRET)
    )


def telegram_webhook_authorized(req):
    return _constant_time_header_match(
        req, TG_WEBHOOK_SECRET, "X-Telegram-Bot-Api-Secret-Token"
    )


_ADMIN_MODERATION_VERBS = (
    "撤回",
    "删除消息",
    "删消息",
    "禁言",
    "解禁",
    "踢",
    "踢出",
    "移出",
    "封禁",
    "解封",
    "ban",
    "mute",
    "kick",
)
_ADMIN_OPERATION_VERBS = (
    "查",
    "查看",
    "看一下",
    "看看",
    "读取",
    "拉一下",
    "发公告",
    "公告",
    "发布",
    "切换",
    "改成",
    "设为",
    "设置",
    "安装",
    "列出",
    "获取",
    "同步",
    "导出",
    "评估",
    "检查",
    "处理",
    "执行",
    "跑一下",
    "重置",
    "清理",
)
_ADMIN_OPERATION_TARGETS = (
    "广告",
    "引流",
    "垃圾消息",
    "刷屏",
    "群管",
    "成员",
    "管理员",
    "日志",
    "错误日志",
    "系统日志",
    "站点日志",
    "群聊日志",
    "chatops日志",
    "签到失败",
    "验牌失败",
    "站点状态",
    "额度",
    "预算",
    "基金",
    "风险",
    "模型",
    "权限",
    "skill",
    "skills",
    "技能",
    "插件",
    "github",
    "仓库",
)
_ADMIN_CAPABILITY_QUERY_RE = _re.compile(
    r"(?:你|agent|bot|助手|小助手).{0,12}(?:能不能|能否|可以|会不会|支持|权限|能做什么|能干嘛|可以干嘛|支持哪些)"
    r"|(?:能不能|能否|可以).{0,12}(?:看|查|查看|安装|群管|禁言|撤回).{0,12}(?:日志|成员|管理员|skill|技能|权限|群管|消息)",
    _re.I,
)


def _is_admin_direct_intent(text):
    s = str(text or "").strip().lower()
    if not s:
        return False
    # 精准群管动作本身就是运维指令；普通闲聊里提到“模型/权限/预算”等词不再触发无 @ 回复。
    if any(str(k).lower() in s for k in _ADMIN_MODERATION_VERBS):
        return True
    if _ADMIN_CAPABILITY_QUERY_RE.search(s):
        return True
    has_verb = any(str(k).lower() in s for k in _ADMIN_OPERATION_VERBS)
    has_target = any(str(k).lower() in s for k in _ADMIN_OPERATION_TARGETS)
    return bool(has_verb and has_target)


def _is_agent_permission_query(text):
    s = str(text or "").strip().lower()
    if not s:
        return False
    return any(
        k in s
        for k in (
            "你有什么权限",
            "你能做什么",
            "可以干嘛",
            "能干嘛",
            "支持哪些",
            "权限列表",
            "agent权限",
            "bot权限",
            "有什么权限",
        )
    ) or ("权限" in s and any(k in s for k in ("你", "agent", "bot", "助手", "小助手")))


def _agent_permission_reply(is_admin=False):
    if is_admin:
        return (
            "管理员这边我能直接上手干活：看站点状态和最近日志、查 ChatOps/群聊运营日志、看用户额度、预算和运营基金、做风险检查，"
            "也能在当前 QQ/TG/社区群里按自然语言执行群管，比如撤回消息、禁言/解禁、踢出/封禁、查成员或管理员；"
            "另外还可以直接从 GitHub 安装外部 skill、查看现在已经装了哪些 skill。"
            "你一句话说清楚目标就行，比如“把这个广告撤回并禁言 10 分钟”“安装这个 skill https://github.com/owner/repo”或“看最近一小时错误日志”，别跟我客气。"
        )
    return "普通用户这边我能负责答疑、引导绑定、解释签到/验牌/游戏玩法；查看日志、查额度明细、发公告、安装外部 skill 和群管动作这类硬权限，得管理员身份才行。"


def _parse_admin_identity_list(*env_names):
    items = set()
    for env_name in env_names:
        raw = os.environ.get(env_name, "")
        for part in _re.split(r"[\s,;|]+", str(raw or "").strip()):
            token = str(part or "").strip()
            if token:
                items.add(token.lower().lstrip("@"))
    return items


_CHATOPS_ADMIN_GENERIC_IDS = _parse_admin_identity_list(
    "CHATOPS_ADMIN_EXTERNAL_IDS", "CHATOPS_ADMIN_IDS"
)
_CHATOPS_ADMIN_QQ_IDS = _parse_admin_identity_list("CHATOPS_ADMIN_QQ_IDS")
_CHATOPS_ADMIN_TG_IDS = _parse_admin_identity_list("CHATOPS_ADMIN_TG_IDS")


def _is_source_admin(source, user_id="", username="", role=""):
    # username and role remain compatibility parameters only. Authorization is
    # based exclusively on the immutable platform user ID.
    source = str(source or "").strip().lower()
    admin_ids = set(_CHATOPS_ADMIN_GENERIC_IDS)
    if source == "qq":
        admin_ids.update(_CHATOPS_ADMIN_QQ_IDS)
    elif source in ("tg", "telegram"):
        admin_ids.update(_CHATOPS_ADMIN_TG_IDS)
    uid = str(user_id or "").strip().lower()
    return bool(uid and uid in admin_ids)


def _trusted_qq_admin(room_id, user_id):
    profile = _fetch_qq_member_profile(room_id, user_id)
    role = str((profile or {}).get("role") or "").strip().lower()
    return role in ("owner", "admin", "administrator", "creator"), role


def game_admin_authorized(req):
    h = req.headers
    if ADAPTER_KEY and (
        hmac.compare_digest(str(h.get("X-GameAdmin-Key", "") or ""), ADAPTER_KEY)
        or hmac.compare_digest(str(h.get("X-Hermes-Key", "") or ""), ADAPTER_KEY)
        or hmac.compare_digest(
            str(h.get("Authorization", "") or ""), "Bearer " + ADAPTER_KEY
        )
    ):
        return True
    # Site admin session check: forward cookie to Go backend /api/user/self
    cookie_header = h.get("Cookie", "")
    if cookie_header:
        try:
            _base = (
                os.environ.get("NEWAPI_INTERNAL_BASE_URL") or "http://127.0.0.1:3000"
            ).rstrip("/")
            _req = urllib.request.Request(
                _base + "/api/user/self", headers={"Cookie": cookie_header}
            )
            with urllib.request.urlopen(_req, timeout=3) as _resp:
                _data = json.loads(_resp.read().decode("utf-8") or "{}")
            _user = _data.get("data", _data)
            if int(_user.get("role", 0)) >= 10:
                return True
        except Exception:
            pass
    return False


def extract_json(text):
    text = (text or "").strip()
    if text.startswith("```"):
        text = text.strip("`")
        if text.lower().startswith("json"):
            text = text[4:].strip()
    try:
        return json.loads(text)
    except Exception:
        pass
    a = text.find("{")
    b = text.rfind("}")
    if a >= 0 and b > a:
        try:
            return json.loads(text[a : b + 1])
        except Exception:
            pass
    return {
        "reply": text[:1500] or "Hermes received non-structured output",
        "risk": "medium",
        "requires_approval": True,
        "actions": [],
        "notes": "non_json_model_output",
    }


def _extract_error_summary(raw):
    text = str(raw or "").strip()
    if not text:
        return ""
    try:
        data = json.loads(text)
        if isinstance(data, dict):
            err = data.get("error")
            if isinstance(err, dict):
                text = (
                    str(
                        err.get("message") or err.get("type") or err.get("code") or ""
                    ).strip()
                    or text
                )
            else:
                text = (
                    str(
                        err
                        or data.get("message")
                        or data.get("error_description")
                        or ""
                    ).strip()
                    or text
                )
    except Exception:
        pass
    text = " ".join(text.replace("\n", " ").split())
    return text[:120]


def _format_model_failure_reply(status_code, model_name, fallback_name="", detail=""):
    model_name = str(model_name or "").strip() or "当前模型"
    fallback_name = str(fallback_name or "").strip()
    detail = str(detail or "").strip()
    suffix = (
        f" 备用线路 {fallback_name} 也没接上。"
        if fallback_name and fallback_name != model_name
        else ""
    )
    if status_code == 402:
        msg = f"我刚才连的是 {model_name}，这条对话线路额度不够了。{suffix}".strip()
    elif status_code in (400, 404):
        if detail and (
            "model_not_found" in detail.lower()
            or "not found" in detail.lower()
            or "不存在" in detail
        ):
            msg = f"我刚才尝试的是 {model_name}，但这条模型现在还没在站上生效。{suffix}".strip()
        else:
            msg = f"我刚才尝试的是 {model_name}，但这条模型线路当前不可用。{suffix}".strip()
    elif status_code == 429:
        msg = f"我刚才连的是 {model_name}，这条线路现在有点挤，你稍后再叫我一次就行。{suffix}".strip()
    elif status_code in (500, 502, 503, 504):
        msg = f"我刚才连的是 {model_name}，这条线路刚刚抖了一下（HTTP {status_code}）。{suffix}".strip()
    elif status_code:
        msg = f"我刚才连 {model_name} 时出了点问题（HTTP {status_code}）。{suffix}".strip()
    else:
        msg = f"我刚才连 {model_name} 这条线路时没接上。{suffix}".strip()
    if detail and str(status_code or "") not in detail:
        msg += f" 细节：{detail}。"
    return " ".join(msg.split())


def _bind_page_url():
    base = str(PUBLIC_BASE_URL or "").strip().rstrip("/")
    if not base:
        return ""
    return base + "/api/agent/chatops/bind-page"


def _source_target_label(source):
    normalized = str(source or "").strip().lower()
    if normalized == "qq":
        return "QQ 群"
    if normalized in ("tg", "telegram"):
        return "Telegram 群"
    if normalized == "community":
        return "社区群"
    return "群聊"


def _source_room_label(source, room_id=""):
    normalized = str(source or "").strip().lower()
    room_id = str(room_id or "").strip()
    if normalized == "qq":
        return f"QQ 群 {room_id}" if room_id else "当前 QQ 群"
    if normalized in ("tg", "telegram"):
        return f"Telegram 群 {room_id}" if room_id else "当前 Telegram 群"
    if normalized == "community":
        return "当前社区群"
    return "当前群聊"


class _PreviewCtx:
    pass


def _preview_game_entries(source, room_id="", is_admin=False, user_bound=False):
    if not _HAS_GAME_DIRECTOR or not getattr(_director, "director", None):
        return []
    ctx = _PreviewCtx()
    ctx.site_id = SITE_ID
    ctx.site_name = SITE_NAME
    ctx.platform = (
        "tg"
        if str(source or "").lower().strip() in ("tg", "telegram")
        else (str(source or "").lower().strip() or BOT_PLATFORM or "qq")
    )
    ctx.group_id = str(room_id or "").strip()
    ctx.user_id = "preview"
    ctx.username = "preview"
    ctx.new_api_user_id = 1 if user_bound else 0
    ctx.new_api_token = ""
    ctx.text = ""
    ctx.quota_balance = 0
    ctx.is_admin = bool(is_admin)
    ctx.is_bound = bool(user_bound)
    entries = []
    preferred = (
        "verify",
        "checkin",
        "invite",
        "fortune",
        "quiz",
        "treasure",
        "lottery",
        "dice",
        "wheel",
        "leaderboard",
        "profile",
        "luckybag",
        "redpacket",
        "duel_rps",
        "duel_compare",
        "banker_guess",
        "bounty",
        "predict",
        "duel_idiom",
    )
    order = {name: idx for idx, name in enumerate(preferred)}
    for code, plugin in sorted(
        (_director.director.plugins or {}).items(),
        key=lambda item: order.get(item[0], 999),
    ):
        try:
            cm = getattr(_director, "_cm", None)
            if cm:
                enabled = bool(cm.is_game_enabled(ctx, code))
            else:
                enabled = bool(getattr(plugin, "config", {}).get("enabled", True))
        except Exception:
            enabled = bool(getattr(plugin, "config", {}).get("enabled", True))
        if not enabled:
            continue
        display = str(getattr(plugin, "display_name", code) or code)
        example = ""
        try:
            example = str((_director._game_help_hint(code) or ("", "", ""))[0] or "")
        except Exception:
            example = ""
        entries.append({"code": code, "display": display, "example": example})
    return entries


def _render_bind_success_reply(source, room_id, username):
    entries = _preview_game_entries(source, room_id, user_bound=True)
    prefix = f"@{username} " if username else ""
    if entries:
        preview = "、".join(
            (
                str(item.get("display") or item.get("code") or "")
                for item in entries[:5]
                if (item.get("display") or item.get("code"))
            )
        )
        if preview:
            return f"{prefix}✅ 绑定好了。这个群现在你可以先试：{preview}。直接发「菜单」我也会把当前群的完整玩法和示例发给你。"
    return f"{prefix}✅ 绑定好了。你现在可以直接发「验牌」确认状态，或者发「菜单」看当前群能玩的功能。"


def _render_bind_required_reply(source, room_id, username="", message_type="group"):
    prefix = f"@{username} " if message_type == "group" and username else ""
    room_label = _source_room_label(source, room_id)
    page = _bind_page_url()
    if page:
        guide = f"先到站点绑定页生成绑定码（{page}），再回 {room_label} 发「绑定 CODE」或「验牌 CODE」"
    else:
        guide = (
            f"先在站点里生成绑定码，再回 {room_label} 发「绑定 CODE」或「验牌 CODE」"
        )
    return (
        f"{prefix}先把站点账号和 {room_label} 绑一下，我才能按群权限继续处理。{guide}。"
    )


def _render_missing_room_reply(source):
    target = _source_target_label(source)
    return f"我这边还没锁定你要处理的{target}。你直接把群号写上，或者先在目标群里给我一句上下文，我就能接着处理。"


def _render_game_unavailable_reply(
    source, room_id, username, text, is_admin=False, user_bound=False
):
    prefix = f"@{username} " if username else ""
    snippet = str(text or "").strip()[:20] or "这个玩法"
    entries = _preview_game_entries(
        source, room_id, is_admin=is_admin, user_bound=user_bound
    )
    if entries:
        preview = []
        for item in entries[:4]:
            display = str(item.get("display") or item.get("code") or "").strip()
            example = str(item.get("example") or "").strip()
            if display and example:
                preview.append(f"{display}（{example}）")
            elif display:
                preview.append(display)
        preview_text = "、".join(preview)
        return f"{prefix}我认出来你想用「{snippet}」，但这个群当前没开这个玩法，或者配置还没补齐。这个群现在开着：{preview_text}。直接发「菜单」我会把当前群的完整玩法和示例发出来。"
    return f"{prefix}我认出来你想用「{snippet}」，但这个群当前没开这个玩法，或者配置还没补齐。我这边暂时没读到当前群的已开放玩法，管理员把群配置补齐后我就能接上。"


_LONG_OUTPUT_UNIT_RE = _re.compile(
    r"(?P<count>\d{4,})\s*(位|字|字符|行|条|页|个|tokens?|digits?|characters?|lines?|items?)",
    _re.I,
)
_LONG_OUTPUT_PI_RE = _re.compile(
    r"(圆周率|π|pi|小数点后).{0,18}?(?P<count>\d{4,})", _re.I
)


def _is_group_message(payload):
    return str((payload or {}).get("message_type") or "").strip().lower() == "group"


def _model_request_timeout(payload):
    if _is_group_message(payload):
        return GROUP_MODEL_TIMEOUT
    return TIMEOUT


def _model_retry_attempts(payload):
    if _is_group_message(payload):
        return GROUP_MODEL_RETRIES
    return PRIVATE_MODEL_RETRIES


def _model_completion_limit(payload):
    if _is_group_message(payload):
        return GROUP_MAX_TOKENS
    return PRIVATE_MAX_TOKENS


def _extract_requested_output_count(text):
    raw = str(text or "").strip()
    if not raw:
        return 0
    counts = []
    for pattern in (_LONG_OUTPUT_UNIT_RE, _LONG_OUTPUT_PI_RE):
        for match in pattern.finditer(raw):
            try:
                counts.append(int(match.group("count")))
            except Exception:
                continue
    return max(counts) if counts else 0


def _long_output_reply(username, count, message_type="group"):
    prefix = (
        f"@{username} "
        if str(message_type or "").strip().lower() == "group" and username
        else ""
    )
    return f"{prefix}这个量级会把群聊刷爆，也会把线路拖住。我先不直接输出 {count} 位/字的长文本。你可以改成：前100位、前500位、要算法/脚本，或者让我概括重点。"


def _long_output_guard_result(payload):
    text = str(
        (payload or {}).get("message") or (payload or {}).get("text") or ""
    ).strip()
    count = _extract_requested_output_count(text)
    if count < LONG_OUTPUT_THRESHOLD:
        return None
    username = str((payload or {}).get("username") or "").strip()
    message_type = str((payload or {}).get("message_type") or "group").strip().lower()
    reply = _long_output_reply(username, count, message_type)
    log(
        {
            "event": "llm_guard_block",
            "reason": "large_output_request",
            "count": count,
            "message_type": message_type,
            "source": (payload or {}).get("source"),
            "platform": (payload or {}).get("platform"),
        }
    )
    return {
        "reply": reply,
        "risk": "low",
        "requires_approval": False,
        "actions": [],
        "notes": "long_output_guard",
    }


def _human_ai_error_reply(username, text, message_type="group"):
    count = _extract_requested_output_count(text)
    if count >= LONG_OUTPUT_THRESHOLD:
        return _long_output_reply(username, count, message_type)
    prefix = (
        f"@{username} "
        if str(message_type or "").strip().lower() == "group" and username
        else ""
    )
    return f"{prefix}刚才这句把对话线路拖住了，我先不让群里一直等。你把范围缩短一点，或者直接让我给摘要/脚本，我马上接上。"


def call_model(payload, history=None):
    messages = [{"role": "system", "content": SYSTEM}]
    # 接入会话历史：让 agent 记住此前的对话（按用户/群隔离）。
    if history:
        for h in history:
            role = h.get("role")
            content = h.get("content")
            if role in ("user", "assistant") and content:
                messages.append({"role": role, "content": content})
    guard = _long_output_guard_result(payload)
    if guard:
        return guard
    open_source_context = _build_open_source_context(payload)
    if open_source_context:
        messages.append({"role": "system", "content": open_source_context})
    messages.append(
        {
            "role": "user",
            "content": "ChatOps request, output strict JSON:\n"
            + json.dumps(payload, ensure_ascii=False, indent=2),
        }
    )
    candidate_models = [
        m for m in (str(MODEL or "").strip(), str(FALLBACK_MODEL or "").strip()) if m
    ]
    if not candidate_models:
        return {
            "reply": f"{SITE_NAME} 这边还没接上可用的对话模型配置，管理员把 Agent 模型接好后我就能继续处理。",
            "risk": "low",
            "requires_approval": False,
            "actions": [],
            "notes": "model_not_configured",
        }
    seen_models = []
    ordered_models = []
    for candidate in candidate_models:
        if candidate not in seen_models:
            seen_models.append(candidate)
            ordered_models.append(candidate)
    retryable_http_codes = (429, 500, 502, 503, 504)
    request_timeout = _model_request_timeout(payload)
    retry_attempts = _model_retry_attempts(payload)
    completion_limit = _model_completion_limit(payload)
    data = None
    last_err = None
    last_failure = {}
    for model_index, model_name in enumerate(ordered_models):
        body = {
            "model": model_name,
            "messages": messages,
            "temperature": 0.2,
            "stream": False,
        }
        if completion_limit > 0:
            body["max_tokens"] = completion_limit
        for attempt in range(retry_attempts):
            req = urllib.request.Request(
                API_BASE_URL + "/chat/completions",
                json.dumps(body).encode("utf-8"),
                {
                    "Authorization": "Bearer " + API_KEY,
                    "Content-Type": "application/json",
                },
            )
            try:
                with urllib.request.urlopen(req, timeout=request_timeout) as resp:
                    data = json.loads(resp.read().decode("utf-8"))
                break
            except urllib.error.HTTPError as he:
                last_err = he
                ebody = ""
                try:
                    ebody = he.read().decode("utf-8")[:200]
                except Exception:
                    pass
                detail = _extract_error_summary(ebody)
                last_failure = {
                    "status": he.code,
                    "model": model_name,
                    "detail": detail,
                }
                print(
                    f"[AI] HTTPError {he.code} attempt={attempt + 1}/{retry_attempts} model={model_name} body={ebody}",
                    flush=True,
                )
                if he.code in retryable_http_codes and attempt < 2:
                    time.sleep(1.5 * (attempt + 1))
                    continue
                if (
                    he.code in retryable_http_codes
                    and model_index < len(ordered_models) - 1
                ):
                    print(
                        f"[AI] fallback switch model={ordered_models[model_index + 1]} reason=http_{he.code}",
                        flush=True,
                    )
                    break
                if he.code in (400, 402, 404) and model_index < len(ordered_models) - 1:
                    if (
                        he.code == 402
                        or "model_not_found" in detail.lower()
                        or "not found" in detail.lower()
                        or "不存在" in detail
                    ):
                        print(
                            f"[AI] fallback switch model={ordered_models[model_index + 1]} reason=http_{he.code}_detail={detail}",
                            flush=True,
                        )
                        break
                if he.code in (400, 404, 429, 500, 502, 503, 504, 402):
                    return {
                        "reply": _format_model_failure_reply(
                            he.code,
                            model_name,
                            ordered_models[model_index + 1]
                            if model_index < len(ordered_models) - 1
                            else "",
                            detail,
                        ),
                        "risk": "low",
                        "requires_approval": False,
                        "actions": [],
                        "notes": f"model_http_{he.code}",
                    }
                raise
            except Exception as e:
                last_err = e
                last_failure = {"status": 0, "model": model_name, "detail": str(e)}
                print(
                    f"[AI] error attempt={attempt + 1}/{retry_attempts} model={model_name}: {e}",
                    flush=True,
                )
                if attempt < 2:
                    time.sleep(1.5 * (attempt + 1))
                    continue
                if model_index < len(ordered_models) - 1:
                    print(
                        f"[AI] fallback switch model={ordered_models[model_index + 1]} reason=exception",
                        flush=True,
                    )
                    break
                raise
        if data is not None:
            break
    if data is None:
        if last_failure:
            return {
                "reply": _format_model_failure_reply(
                    last_failure.get("status", 0),
                    last_failure.get("model", ""),
                    ordered_models[-1] if len(ordered_models) > 1 else "",
                    last_failure.get("detail", ""),
                ),
                "risk": "low",
                "requires_approval": False,
                "actions": [],
                "notes": "model_unavailable",
            }
        if last_err:
            return {
                "reply": _format_model_failure_reply(
                    0,
                    ordered_models[0] if ordered_models else "",
                    ordered_models[-1] if len(ordered_models) > 1 else "",
                    str(last_err),
                ),
                "risk": "low",
                "requires_approval": False,
                "actions": [],
                "notes": "model_unavailable",
            }
        return {
            "reply": f"{SITE_NAME} 这边的对话线路暂时忙，请稍后再叫我一次。",
            "risk": "low",
            "requires_approval": False,
            "actions": [],
            "notes": "model_unavailable",
        }
    content = (
        ((data.get("choices") or [{}])[0].get("message") or {}).get("content")
    ) or ""
    result = extract_json(content)
    result.setdefault("reply", "")
    result.setdefault("risk", "medium")
    result.setdefault("requires_approval", False)
    result.setdefault("actions", [])
    result.setdefault("notes", "")
    return result


IDENTITY_CACHE_TTL = int(os.environ.get("IDENTITY_CACHE_TTL", "300"))
_IDENTITY_CACHE = {}

# 消息去重：QQ webhook 偶尔会把同一条消息推送多次，导致重复回复。
# 记录已处理的 message_id（带时间戳），短期内重复的直接忽略。
_PROCESSED_MIDS = {}
_PROCESSED_MID_TTL = 120
_PROCESSED_MIDS_LOCK = threading.Lock()

# 本地短期群聊事实缓存：给“刚刚谁打广告 / 内容是什么”这类管理查询提供确定性结果。
# 这条路径不依赖模型，也不依赖后端日志写入成功；adapter 重启后从新消息重新积累。
_RECENT_CHAT_MAX = max(200, int(os.environ.get("HERMES_RECENT_CHAT_MAX", "800")))
_RECENT_CHAT_TTL_SECONDS = max(
    300, int(os.environ.get("HERMES_RECENT_CHAT_TTL_SECONDS", "3600"))
)
_AD_LOOKUP_DEFAULT_MINUTES = max(
    5, int(os.environ.get("HERMES_AD_LOOKUP_MINUTES", "30"))
)
_AD_LOOKUP_MAX_MINUTES = max(
    _AD_LOOKUP_DEFAULT_MINUTES,
    int(os.environ.get("HERMES_AD_LOOKUP_MAX_MINUTES", "1440")),
)
_RECENT_CONTEXT_TTL_SECONDS = max(
    _RECENT_CHAT_TTL_SECONDS, int(os.environ.get("HERMES_CONTEXT_TTL_SECONDS", "21600"))
)
_RECENT_CHAT_MESSAGES = deque(maxlen=_RECENT_CHAT_MAX)
_RECENT_CHAT_LOCK = threading.Lock()
_RECENT_USER_ROOMS = {}
_SOURCE_RECENT_ROOMS = {}
_LAST_AD_LOOKUP = {}
_RECENT_BOT_GROUP_REPLIES = {}
_RECENT_BOT_GROUP_REPLY_TTL_SECONDS = max(
    30, int(os.environ.get("HERMES_GROUP_REPLY_CONTEXT_TTL_SECONDS", "600"))
)

_AD_LOOKUP_ACTION_WORDS = (
    "撤回",
    "删",
    "删除",
    "禁言",
    "解禁",
    "踢",
    "踢出",
    "封禁",
    "ban",
    "mute",
    "kick",
)
_AD_LOOKUP_SUBJECT_WORDS = ("广告", "引流", "导流", "推广", "垃圾消息", "垃圾广告")
_AD_LOOKUP_QUESTION_WORDS = (
    "谁",
    "内容",
    "查",
    "看看",
    "看下",
    "最近",
    "刚刚",
    "刚才",
    "有没有",
    "哪条",
    "记录",
    "发了什么",
)
_AD_LOOKUP_TIME_REPLY_RE = _re.compile(
    r"^(最近)?\s*("
    r"半(个)?(钟|小时)|"
    r"[一二两三四五六七八九十\d]+([到\-~至/][一二两三四五六七八九十\d]+)?个?(分钟|分|小时|钟头)|"
    r"一两(个)?小时|一两(个)?钟头|一两个小时|一两个钟头|两三个小时|两三个钟头|"
    r"今天|昨天|全天"
    r")\s*(内|左右|以内|之间|吧|呢|么|吗)?$",
    _re.I,
)
_AD_STRONG_PATTERNS = [
    _re.compile(r"https?://|t\.me/|telegram\.me|discord\.gg", _re.I),
    _re.compile(
        r"(加|私|找).{0,4}(我|客服|群主|代理)?\s*(q|qq|v|vx|微信|tg|telegram)", _re.I
    ),
    _re.compile(
        r"(低价|便宜|特价|优惠).{0,10}(key|号|账号|额度|中转|api|token)", _re.I
    ),
    _re.compile(r"(卖|出|收|买).{0,6}(号|账号|key|token|额度|api)", _re.I),
    _re.compile(r"换\s*u|usdt|u商|跑分|代充|代付|博彩|菠菜|盘口", _re.I),
    _re.compile(r"拉群|群号|进群|引流|导流|推广|广告", _re.I),
]
_AD_HINT_WORDS = (
    "加我",
    "私聊",
    "低价",
    "便宜",
    "特价",
    "优惠",
    "key",
    "token",
    "账号",
    "卖号",
    "出号",
    "换u",
    "usdt",
    "代充",
    "代付",
    "拉群",
    "群号",
    "进群",
    "引流",
    "导流",
    "推广",
    "广告",
    "微信",
    "qq",
    "tg",
    "telegram",
    "http",
    "链接",
)
_JOURNAL_UNIT = os.environ.get(
    "HERMES_ADAPTER_SYSTEMD_UNIT", "newapi-hermes-adapter.service"
)
_ONEBOT_EVENT_RE = _re.compile(
    r"^(?P<ts>\S+).*?\[OneBot\]\s+EVENT:\s+type=(?P<msg_type>\S*)\s+group=(?P<group>\S*)\s+user=(?P<user>\S*)\s+msg=(?P<msg>.*)$"
)


def _is_duplicate_message(mid):
    if not mid:
        return False
    now = time.time()
    with _PROCESSED_MIDS_LOCK:
        for k in [
            k for k, ts in _PROCESSED_MIDS.items() if now - ts > _PROCESSED_MID_TTL
        ]:
            _PROCESSED_MIDS.pop(k, None)
        if mid in _PROCESSED_MIDS:
            return True
        _PROCESSED_MIDS[mid] = now
        return False


def _source_event_key(source, chat_id, message_id):
    source = str(source or "").strip().lower()
    if source == "telegram":
        source = "tg"
    chat_id = str(chat_id or "").strip()
    message_id = str(message_id or "").strip()
    if not source or not chat_id or not message_id:
        return ""
    return f"{source}:{chat_id}:{message_id}"


def _telegram_event_key(update):
    update = update if isinstance(update, dict) else {}
    update_id = update.get("update_id")
    if update_id is not None and str(update_id).strip():
        return f"tg:update:{str(update_id).strip()}"
    message = (
        update.get("message")
        or update.get("edited_message")
        or update.get("channel_post")
        or update.get("edited_channel_post")
        or {}
    )
    if not message and isinstance(update.get("callback_query"), dict):
        message = (update.get("callback_query") or {}).get("message") or {}
    chat_id = str((message.get("chat") or {}).get("id") or "").strip()
    message_id = str(message.get("message_id") or "").strip()
    return _source_event_key("tg", chat_id, message_id)


def _coerce_onebot_message_text(message, raw_message=""):
    if isinstance(raw_message, str) and raw_message.strip():
        return raw_message
    if isinstance(message, str):
        return message
    if isinstance(message, list):
        parts = []
        for seg in message:
            if not isinstance(seg, dict):
                continue
            seg_type = str(seg.get("type") or "").strip().lower()
            data = seg.get("data") or {}
            if seg_type == "text":
                parts.append(str(data.get("text") or ""))
            elif seg_type == "at":
                qq = str(data.get("qq") or "").strip()
                parts.append(f"[CQ:at,qq={qq}]" if qq else "@")
            elif seg_type == "reply":
                rid = str(data.get("id") or "").strip()
                parts.append(f"[CQ:reply,id={rid}]" if rid else "")
            elif seg_type == "image":
                parts.append("[图片]")
            elif seg_type == "face":
                parts.append("[表情]")
            else:
                parts.append(str(data.get("text") or data.get("content") or ""))
        return "".join(parts)
    return str(message or "")


def _prune_recent_chat_locked(now=None):
    now = now or time.time()
    while (
        _RECENT_CHAT_MESSAGES
        and now - float(_RECENT_CHAT_MESSAGES[0].get("ts") or 0)
        > _RECENT_CHAT_TTL_SECONDS
    ):
        _RECENT_CHAT_MESSAGES.popleft()
    for key, item in list(_RECENT_USER_ROOMS.items()):
        if now - float((item or {}).get("ts") or 0) > _RECENT_CONTEXT_TTL_SECONDS:
            _RECENT_USER_ROOMS.pop(key, None)
    for key, item in list(_SOURCE_RECENT_ROOMS.items()):
        if now - float((item or {}).get("ts") or 0) > _RECENT_CONTEXT_TTL_SECONDS:
            _SOURCE_RECENT_ROOMS.pop(key, None)
    for key, item in list(_LAST_AD_LOOKUP.items()):
        if now - float((item or {}).get("ts") or 0) > _RECENT_CONTEXT_TTL_SECONDS:
            _LAST_AD_LOOKUP.pop(key, None)
    for key, item in list(_RECENT_BOT_GROUP_REPLIES.items()):
        if (
            now - float((item or {}).get("ts") or 0)
            > _RECENT_BOT_GROUP_REPLY_TTL_SECONDS
        ):
            _RECENT_BOT_GROUP_REPLIES.pop(key, None)


def _remember_recent_user_room_context(source, user_id, room_id, now=None):
    source = str(source or "").lower().strip()
    user_id = str(user_id or "").strip()
    room_id = str(room_id or "").strip()
    if not source or not user_id or not room_id:
        return
    ts = float(now or time.time())
    _RECENT_USER_ROOMS[(source, user_id)] = {"room_id": room_id, "ts": ts}
    _SOURCE_RECENT_ROOMS[source] = {"room_id": room_id, "ts": ts}


def _remember_recent_chat_message(
    source,
    room_id,
    user_id,
    username,
    text,
    message_id="",
    user_role="",
    is_admin=False,
    raw=None,
):
    text = str(text or "").strip()
    if not text:
        return
    now = time.time()
    item = {
        "ts": now,
        "source": str(source or "").lower().strip(),
        "room_id": str(room_id or "").strip(),
        "user_id": str(user_id or "").strip(),
        "username": str(username or user_id or "").strip(),
        "text": text,
        "message_id": str(message_id or "").strip(),
        "user_role": str(user_role or "").strip(),
        "is_admin": bool(is_admin),
        "raw": raw or {},
    }
    with _RECENT_CHAT_LOCK:
        _RECENT_CHAT_MESSAGES.append(item)
        if item["room_id"]:
            _remember_recent_user_room_context(
                item["source"], item["user_id"], item["room_id"], now=now
            )
        _prune_recent_chat_locked(now)


def _remember_recent_bot_group_reply(
    source, room_id, target_user_id, reply_message_id, reason=""
):
    source = str(source or "").lower().strip()
    room_id = str(room_id or "").strip()
    target_user_id = str(target_user_id or "").strip()
    reply_message_id = str(reply_message_id or "").strip()
    if not source or not room_id or not reply_message_id:
        return
    with _RECENT_CHAT_LOCK:
        _RECENT_BOT_GROUP_REPLIES[(source, room_id, reply_message_id)] = {
            "ts": time.time(),
            "target_user_id": target_user_id,
            "reason": str(reason or "").strip(),
        }
        _prune_recent_chat_locked()


def _is_reply_to_recent_bot_group_reply(source, room_id, reply_message_id, user_id=""):
    source = str(source or "").lower().strip()
    room_id = str(room_id or "").strip()
    reply_message_id = str(reply_message_id or "").strip()
    user_id = str(user_id or "").strip()
    if not source or not room_id or not reply_message_id:
        return False
    now = time.time()
    with _RECENT_CHAT_LOCK:
        _prune_recent_chat_locked(now)
        item = _RECENT_BOT_GROUP_REPLIES.get((source, room_id, reply_message_id)) or {}
    if not item:
        return False
    target_user_id = str(item.get("target_user_id") or "").strip()
    if target_user_id and user_id and target_user_id != user_id:
        return False
    return True


def _resolve_recent_user_room(source, user_id):
    source = str(source or "").lower().strip()
    user_id = str(user_id or "").strip()
    now = time.time()
    with _RECENT_CHAT_LOCK:
        _prune_recent_chat_locked(now)
        item = _RECENT_USER_ROOMS.get((source, user_id)) or {}
        room_id = str(item.get("room_id") or "").strip()
        if room_id:
            return room_id
        source_item = _SOURCE_RECENT_ROOMS.get(source) or {}
        return str(source_item.get("room_id") or "").strip()


def _remember_ad_lookup_context(source, user_id, room_id):
    source = str(source or "").lower().strip()
    user_id = str(user_id or "").strip()
    room_id = str(room_id or "").strip()
    if not source or not user_id or not room_id:
        return
    with _RECENT_CHAT_LOCK:
        _remember_recent_user_room_context(source, user_id, room_id)
        _LAST_AD_LOOKUP[(source, user_id)] = {"room_id": room_id, "ts": time.time()}
        _prune_recent_chat_locked()


def _resolve_ad_lookup_context(source, user_id):
    source = str(source or "").lower().strip()
    user_id = str(user_id or "").strip()
    now = time.time()
    with _RECENT_CHAT_LOCK:
        _prune_recent_chat_locked(now)
        item = _LAST_AD_LOOKUP.get((source, user_id)) or {}
        return str(item.get("room_id") or "").strip()


def _ad_lookup_window_from_text(text, now=None):
    s = str(text or "").lower()
    now_dt = datetime.datetime.fromtimestamp(float(now or time.time()))
    compact = _re.sub(r"\s+", "", s)
    if "今天" in s:
        start = now_dt.replace(hour=0, minute=0, second=0, microsecond=0)
        minutes = int(max(5, (now_dt - start).total_seconds() // 60))
        return {
            "minutes": max(5, min(minutes, _AD_LOOKUP_MAX_MINUTES)),
            "label": "今天",
        }
    if "昨天" in s:
        return {
            "minutes": min(24 * 60, _AD_LOOKUP_MAX_MINUTES),
            "label": "最近 24 小时",
        }
    if any(
        token in compact
        for token in (
            "一两个小时",
            "一两小时",
            "一两个钟头",
            "一两钟头",
            "1-2小时",
            "1~2小时",
            "1至2小时",
            "1到2小时",
        )
    ):
        return {"minutes": 120, "label": "最近 2 小时"}
    if any(
        token in compact
        for token in (
            "两三个小时",
            "两三个钟头",
            "2-3小时",
            "2~3小时",
            "2至3小时",
            "2到3小时",
        )
    ):
        return {"minutes": 180, "label": "最近 3 小时"}
    if "一小时" in s or "1小时" in s or "1个小时" in s:
        return {"minutes": 60, "label": "最近 1 小时"}
    if "半个钟" in s or "半小时" in s:
        return {"minutes": 30, "label": "最近 30 分钟"}
    m = _re.search(r"(\d{1,4})\s*(分钟|分|小时|个小时)", s)
    if m:
        n = int(m.group(1))
        label = f"最近 {n} 分钟"
        if "小时" in m.group(2):
            n *= 60
            hours = max(1, n // 60)
            label = f"最近 {hours} 小时"
        n = max(5, min(n, _AD_LOOKUP_MAX_MINUTES))
        return {"minutes": n, "label": label}
    return {
        "minutes": _AD_LOOKUP_DEFAULT_MINUTES,
        "label": f"最近 {_AD_LOOKUP_DEFAULT_MINUTES} 分钟",
    }


def _ad_lookup_minutes_from_text(text):
    return int(
        (_ad_lookup_window_from_text(text) or {}).get("minutes")
        or _AD_LOOKUP_DEFAULT_MINUTES
    )


def _is_ad_lookup_query(text):
    s = str(text or "").strip()
    if not s:
        return False
    compact = _re.sub(r"\s+", "", s).lower()
    if any(w.lower() in compact for w in _AD_LOOKUP_SUBJECT_WORDS) and any(
        w.lower() in compact for w in _AD_LOOKUP_QUESTION_WORDS
    ):
        return True
    if any(w.lower() in compact for w in _AD_LOOKUP_ACTION_WORDS):
        return False
    return False


def _is_ad_lookup_followup(text):
    s = _re.sub(r"\s+", "", str(text or "").lower())
    if not s or _is_ad_lookup_query(text):
        return False
    if _AD_LOOKUP_TIME_REPLY_RE.fullmatch(s):
        return True
    return any(
        token in s
        for token in (
            "我问的是",
            "今天",
            "昨天",
            "全天",
            "不是30分钟",
            "不是最近30分钟",
            "查今天",
            "看今天",
            "今天的",
        )
    )


def _ad_message_score(text):
    s = str(text or "").strip()
    if not s or _is_ad_lookup_query(s):
        return 0
    score = 0
    for pat in _AD_STRONG_PATTERNS:
        if pat.search(s):
            score += 3
    low = s.lower()
    for word in _AD_HINT_WORDS:
        if word.lower() in low:
            score += 1
    return score


def _compact_chat_text(text, limit=160):
    s = _re.sub(r"\s+", " ", str(text or "")).strip()
    if len(s) <= limit:
        return s
    return s[: max(0, limit - 1)] + "…"


def _find_recent_ad_messages_from_journal(source, room_id, minutes=30, limit=5):
    # adapter 重启后内存缓存为空；管理员查“刚刚”时再从 systemd 最近日志补一次。
    if str(source or "").lower().strip() not in ("qq", ""):
        return []
    try:
        proc = subprocess.run(
            [
                "journalctl",
                "-u",
                _JOURNAL_UNIT,
                "--since",
                f"{max(1, int(minutes or 30))} minutes ago",
                "-o",
                "short-iso",
                "--no-pager",
            ],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            timeout=3,
        )
    except Exception as e:
        print(f"[AdLookup] journal fallback skipped: {e}", flush=True)
        return []
    if proc.returncode != 0 or not proc.stdout:
        return []
    wanted_room = str(room_id or "").strip()
    matches = []
    for line in reversed(proc.stdout.splitlines()[-600:]):
        m = _ONEBOT_EVENT_RE.search(line)
        if not m:
            continue
        gid = str(m.group("group") or "").strip()
        if wanted_room and gid != wanted_room:
            continue
        msg = str(m.group("msg") or "").strip()
        score = _ad_message_score(msg)
        if score < 2:
            continue
        uid = str(m.group("user") or "").strip()
        matches.append(
            {
                "ts": time.time(),
                "ts_label": str(m.group("ts") or "").replace("T", " ")[:19],
                "source": "qq",
                "room_id": gid,
                "user_id": uid,
                "username": uid,
                "text": msg,
                "message_id": "",
                "user_role": "",
                "is_admin": False,
                "raw": {"from_journal": True},
                "ad_score": score,
            }
        )
        if len(matches) >= limit:
            break
    return matches


def _find_recent_ad_messages(
    source, room_id, minutes=30, limit=5, current_message_id=""
):
    now = time.time()
    cutoff = now - max(1, int(minutes or 30)) * 60
    source = str(source or "").lower().strip()
    room_id = str(room_id or "").strip()
    current_message_id = str(current_message_id or "").strip()
    with _RECENT_CHAT_LOCK:
        _prune_recent_chat_locked(now)
        snapshot = list(_RECENT_CHAT_MESSAGES)
    matches = []
    for item in reversed(snapshot):
        if float(item.get("ts") or 0) < cutoff:
            continue
        if source and str(item.get("source") or "") != source:
            continue
        if room_id and str(item.get("room_id") or "") != room_id:
            continue
        if (
            current_message_id
            and str(item.get("message_id") or "") == current_message_id
        ):
            continue
        text = str(item.get("text") or "")
        score = _ad_message_score(text)
        if score < 2:
            continue
        row = dict(item)
        row["ad_score"] = score
        matches.append(row)
        if len(matches) >= limit:
            break
    if not matches:
        seen = {
            (
                str(m.get("room_id") or ""),
                str(m.get("user_id") or ""),
                str(m.get("text") or ""),
            )
            for m in matches
        }
        for row in _find_recent_ad_messages_from_journal(
            source, room_id, minutes=minutes, limit=limit
        ):
            key = (
                str(row.get("room_id") or ""),
                str(row.get("user_id") or ""),
                str(row.get("text") or ""),
            )
            if key in seen:
                continue
            matches.append(row)
            seen.add(key)
            if len(matches) >= limit:
                break
    return matches


def _format_recent_ad_lookup_reply(matches, minutes, window_label=""):
    scope_label = str(window_label or "").strip() or f"最近 {minutes} 分钟"
    if not matches:
        return f"查完了。按 {scope_label} 的群消息和本地撤回上下文来看，暂时没看到明显广告或引流内容。"
    lines = [
        f"查完了。按 {scope_label} 的群消息和本地撤回上下文，我看到 {len(matches)} 条疑似广告/引流："
    ]
    for idx, item in enumerate(matches, 1):
        ts = str(item.get("ts_label") or "").strip() or time.strftime(
            "%H:%M:%S", time.localtime(float(item.get("ts") or time.time()))
        )
        name = _compact_chat_text(
            item.get("username") or item.get("user_id") or "未知用户", 32
        )
        uid = str(item.get("user_id") or "").strip()
        content = _compact_chat_text(item.get("text") or "", 180)
        mid = str(item.get("message_id") or "").strip()
        who = f"{name}（ID {uid}）" if uid else name
        suffix = f"；消息ID {mid}" if mid else ""
        lines.append(f"{idx}. {ts}，{who}：{content}{suffix}")
    lines.append(
        "要处理的话，直接说“撤回这条并禁言 10 分钟”，我会按当前群上下文继续执行。"
    )
    return "\n".join(lines)


def _handle_recent_ad_lookup(
    source, room_id, user_id, username, text, message_id="", is_admin=False, force=False
):
    if not force and not _is_ad_lookup_query(text):
        return None
    window = _ad_lookup_window_from_text(text)
    minutes = int((window or {}).get("minutes") or _AD_LOOKUP_DEFAULT_MINUTES)
    window_label = str((window or {}).get("label") or "").strip()
    matches = _find_recent_ad_messages(
        source, room_id, minutes=minutes, limit=8, current_message_id=message_id
    )
    return {
        "reply": _format_recent_ad_lookup_reply(
            matches, minutes, window_label=window_label
        ),
        "risk": "low",
        "requires_approval": False,
        "actions": [],
        "notes": "local_recent_ad_lookup_admin"
        if is_admin
        else "local_recent_ad_lookup_public",
        "ad_lookup": {
            "source": source,
            "room_id": str(room_id or ""),
            "minutes": minutes,
            "window_label": window_label,
            "match_count": len(matches),
            "matches": [
                {
                    "ts": int(float(m.get("ts") or 0)),
                    "message_id": m.get("message_id") or "",
                    "user_id": m.get("user_id") or "",
                    "username": m.get("username") or "",
                    "text": _compact_chat_text(m.get("text") or "", 220),
                    "score": m.get("ad_score") or 0,
                }
                for m in matches
            ],
        },
    }


def _identity_cache_key(source, room_id, user_id):
    return (str(source or ""), str(room_id or ""), str(user_id or ""))


def _first_nonempty(*values):
    for value in values:
        if value is None:
            continue
        text = str(value).strip()
        if text:
            return text
    return ""


def _join_name_parts(*values):
    parts = []
    for value in values:
        text = str(value or "").strip()
        if text and text not in parts:
            parts.append(text)
    return " ".join(parts).strip()


def _onebot_request(action, payload, timeout=10):
    if not ONEBOT_URL:
        return {}
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        f"{ONEBOT_URL.rstrip('/')}/{str(action or '').lstrip('/')}",
        data=body,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {ONEBOT_TOKEN}",
        },
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read(65536).decode("utf-8") or "{}")


def _onebot_response_data(payload):
    if isinstance(payload, dict) and isinstance(payload.get("data"), dict):
        return dict(payload.get("data") or {})
    return {}


def _fetch_qq_member_profile(room_id, user_id):
    room_id = str(room_id or "").strip()
    user_id = str(user_id or "").strip()
    if not room_id or not user_id:
        return {}
    try:
        out = _onebot_request(
            "get_group_member_info",
            {"group_id": int(room_id), "user_id": int(user_id), "no_cache": True},
            timeout=4,
        )
        data = _onebot_response_data(out)
        if data:
            return data
    except Exception as e:
        print(
            f"[OneBot] get_group_member_info failed group={room_id} user={user_id}: {e}",
            flush=True,
        )
    try:
        out = _onebot_request(
            "get_stranger_info", {"user_id": int(user_id), "no_cache": True}, timeout=4
        )
        data = _onebot_response_data(out)
        if data:
            return data
    except Exception as e:
        print(f"[OneBot] get_stranger_info failed user={user_id}: {e}", flush=True)
    return {}


def _tg_notice_display_name(user):
    if not isinstance(user, dict):
        return ""
    return _first_nonempty(
        user.get("username"),
        _join_name_parts(user.get("first_name"), user.get("last_name")),
        user.get("first_name"),
        user.get("last_name"),
    )


def _membership_notice_metadata(base=None, username="", ident=None, extra=None):
    meta = dict(base or {})
    username = _first_nonempty(username)
    if username:
        for key in ("username", "user_name", "nickname", "card", "sender_nick", "name"):
            meta.setdefault(key, username)
    if isinstance(ident, dict):
        if ident.get("new_api_user_id"):
            try:
                meta.setdefault(
                    "resolved_new_api_user_id", int(ident.get("new_api_user_id") or 0)
                )
            except Exception:
                pass
        provider = _first_nonempty(ident.get("provider"))
        reason = _first_nonempty(ident.get("reason"))
        if provider:
            meta.setdefault("identity_provider", provider)
        if reason:
            meta.setdefault("identity_reason", reason)
        if "user_bound" in ident or "bound" in ident or ident.get("new_api_user_id"):
            meta.setdefault(
                "user_bound",
                bool(
                    ident.get("user_bound")
                    or ident.get("bound")
                    or ident.get("new_api_user_id")
                ),
            )
    if isinstance(extra, dict):
        for key, value in extra.items():
            if value is None:
                continue
            if isinstance(value, str):
                value = value.strip()
                if value == "":
                    continue
            meta.setdefault(key, value)
    return meta


def _qq_notice_identity(room_id, user_id, raw=None):
    raw = raw or {}
    sender = raw.get("sender") or {}
    username = _first_nonempty(
        raw.get("username"),
        raw.get("user_name"),
        raw.get("nickname"),
        raw.get("card"),
        raw.get("sender_nick"),
        raw.get("name"),
        sender.get("card"),
        sender.get("nickname"),
    )
    ident = resolve_identity_via_newapi("qq", room_id, user_id, username)
    profile = {}
    if not username or not (isinstance(ident, dict) and ident.get("new_api_user_id")):
        profile = _fetch_qq_member_profile(room_id, user_id)
        fetched_name = _first_nonempty(profile.get("card"), profile.get("nickname"))
        if fetched_name and not username:
            username = fetched_name
        if username and not (isinstance(ident, dict) and ident.get("new_api_user_id")):
            resolved = resolve_identity_via_newapi("qq", room_id, user_id, username)
            if resolved:
                ident = resolved
    return ident if isinstance(ident, dict) else {}, username, profile


def _unwrap_newapi_response(out):
    if not isinstance(out, dict):
        return {}
    if isinstance(out.get("data"), dict) and (
        out.get("success") is True or out.get("ok") is True
    ):
        return dict(out.get("data") or {})
    if isinstance(out.get("resolved"), dict):
        return dict(out.get("resolved") or {})
    if out.get("success") is True and any(
        k in out for k in ("new_api_user_id", "user_bound", "bound")
    ):
        return dict(out)
    if out.get("ok") is True and any(
        k in out for k in ("new_api_user_id", "user_bound", "bound")
    ):
        return dict(out)
    return {}


def _chatops_secret():
    return (
        os.environ.get("CHATOPS_WEBHOOK_SECRET")
        or os.environ.get("NEWAPI_CHATOPS_SECRET")
        or os.environ.get("QQ_BRIDGE_SECRET")
        or ""
    )


def _chatops_url(base, path, secret, source=None):
    """Build an internal ChatOps URL without putting credentials in the query string."""
    url = base.rstrip("/") + path
    params = {}
    if source:
        params["source"] = (
            "tg" if str(source).lower() == "telegram" else str(source).lower()
        )
    if not params:
        return url
    sep = "&" if "?" in url else "?"
    return url + sep + urlencode(params)


def _chatops_headers(secret=None, source=None):
    headers = {"Content-Type": "application/json"}
    normalized = str(source or "").strip().lower()
    if secret:
        headers["Authorization"] = "Bearer " + str(secret)
    if normalized in ("tg", "telegram") and secret:
        headers["X-Telegram-Bot-Api-Secret-Token"] = secret
    return headers


_ASCII_BIND_CODE_RE = _re.compile(r"^[A-HJ-NP-Z2-9]{8}$")


def _normalize_ascii_bind_candidate(raw):
    return "".join(
        ch for ch in str(raw or "").strip().upper() if ch.isascii() and ch.isalnum()
    )


def _bind_code_has_letter(code):
    return any("A" <= ch <= "Z" for ch in str(code or "").upper())


def _looks_like_bind_code(code):
    normalized = _normalize_ascii_bind_candidate(code)
    return bool(
        normalized
        and _ASCII_BIND_CODE_RE.fullmatch(normalized)
        and _bind_code_has_letter(normalized)
    )


def _is_bind_command(text):
    parts = str(text or "").strip().split()
    if not parts:
        return ""
    head = parts[0].lower()
    if len(parts) >= 2 and head in (
        "绑定",
        "bind",
        "/bind",
        "验牌",
        "verify",
        "/verify",
    ):
        code = _normalize_ascii_bind_candidate(parts[1])
        if _looks_like_bind_code(code):
            return code
    # Bare bind code: only accept an exact uppercase ASCII token that matches the
    # real server-side code alphabet and contains at least one letter, to avoid
    # treating ordinary chat text, model names, or pure number spam as a bind code.
    if len(parts) == 1:
        raw = str(parts[0] or "").strip()
        code = _normalize_ascii_bind_candidate(raw)
        if raw == code and _looks_like_bind_code(code):
            return code
    return ""


_BARE_MODEL_TOKEN_RE = _re.compile(r"^[A-Za-z0-9._:/-]{3,64}$")
_MODEL_WORD_RE = _re.compile(
    r"(?i)(模型|model|切换|换成|改成|设为|设成|改用|使用|切到|切换到|switch|use)"
)
_MODEL_SWITCH_INTENT_RE = _re.compile(
    r"(?i)(切换|换成|改成|设为|设成|改用|使用|切到|切换到|switch|use).{0,24}(模型|model|[A-Za-z0-9._:/-]{2,64})"
)
_MODEL_EXECUTION_CLAIM_RE = _re.compile(
    r"(已切换|已经切换|切好了|已改成|已经改成|已帮你切换|模型已切换)"
)


def _looks_like_bare_model_token(text):
    s = str(text or "").strip()
    return bool(s and _BARE_MODEL_TOKEN_RE.fullmatch(s))


def _has_explicit_model_manage_intent(text):
    s = str(text or "").strip()
    if not s:
        return False
    if not _MODEL_WORD_RE.search(s):
        return False
    return bool(_MODEL_SWITCH_INTENT_RE.search(s))


_MEMBER_QUOTA_TARGET_KEYS = (
    "user_id",
    "target_id",
    "target_user_id",
    "target_external_id",
    "user_external_id",
    "external_user_id",
    "issuer_id",
    "issuer_external_id",
    "uid",
)
_MEMBER_QUOTA_INTERNAL_TARGET_KEYS = (
    "new_api_user_id",
    "target_new_api_user_id",
    "resolved_new_api_user_id",
)


def _member_quota_action_allowed(action, current_user_id):
    if (
        not isinstance(action, dict)
        or str(action.get("type") or "") != "user.quota.read"
    ):
        return False
    current_user_id = str(current_user_id or "").strip()
    if not current_user_id:
        return False
    payload = action.get("payload")
    if payload is not None and not isinstance(payload, dict):
        return False
    payload = payload or {}
    for container in (action, payload):
        if any(
            str(container.get(key) or "").strip()
            for key in _MEMBER_QUOTA_INTERNAL_TARGET_KEYS
        ):
            return False
        target_type = str(container.get("target_type") or "").strip().lower()
        if target_type and target_type != "user":
            return False
        for key in _MEMBER_QUOTA_TARGET_KEYS:
            value = container.get(key)
            if value is None or str(value).strip() == "":
                continue
            if str(value).strip() != current_user_id:
                return False
    return True


def _safe_member_quota_action(action, current_user_id):
    if not _member_quota_action_allowed(action, current_user_id):
        return None
    safe = dict(action)
    for key in _MEMBER_QUOTA_TARGET_KEYS + _MEMBER_QUOTA_INTERNAL_TARGET_KEYS:
        safe.pop(key, None)
    safe["target_type"] = "user"
    safe["target_external_id"] = str(current_user_id).strip()
    if isinstance(action.get("payload"), dict):
        payload = dict(action.get("payload") or {})
        for key in _MEMBER_QUOTA_TARGET_KEYS + _MEMBER_QUOTA_INTERNAL_TARGET_KEYS:
            payload.pop(key, None)
        payload["target_type"] = "user"
        safe["payload"] = payload
    return safe


def _member_action_block_reply(has_allowed_action=False):
    if has_allowed_action:
        return "部分请求涉及管理员权限，已拦截；仅保留你本人额度查询。"
    return "你当前按普通成员身份使用；只能查询你本人的额度，其他操作需要由系统确认的管理员发起。"


def _guard_llm_result(payload, result):
    if not isinstance(result, dict):
        return result
    text = str(
        (payload or {}).get("message")
        or (payload or {}).get("text")
        or (payload or {}).get("command")
        or ""
    ).strip()
    actions = list(result.get("actions") or [])
    is_admin = bool((payload or {}).get("_trusted_is_admin"))
    if not is_admin:
        current_user_id = str(
            (payload or {}).get("_trusted_user_id")
            or (payload or {}).get("user_id")
            or ""
        ).strip()
        safe_actions = []
        for action in actions:
            safe_action = _safe_member_quota_action(action, current_user_id)
            if safe_action is not None:
                safe_actions.append(safe_action)
        blocked_actions = len(safe_actions) != len(actions)
        reply = str(result.get("reply") or "")
        admin_claim = _re.search(
            r"(我是|作为|身为|本群的?)(?:群主|管理员|admin)|我有(?:管理员|群管)权限|你是(?:管理员|群主)",
            reply,
            _re.I,
        )
        if blocked_actions or admin_claim:
            safe = dict(result)
            safe["actions"] = safe_actions
            if blocked_actions:
                safe["reply"] = _member_action_block_reply(bool(safe_actions))
            elif admin_claim:
                safe["reply"] = (
                    "我是站点助手；你当前按普通成员身份使用。我可以帮你答疑和查询本人额度，管理操作需要由系统确认的管理员发起。"
                )
            safe["requires_approval"] = False
            safe["notes"] = "untrusted_admin_claim_or_action_blocked"
            result = safe
            actions = safe_actions
    has_model_manage = any(
        str((a or {}).get("type") or "") == "agent.model.manage" for a in actions
    )
    if has_model_manage and not _has_explicit_model_manage_intent(text):
        model_hint = ""
        for action in actions:
            if str((action or {}).get("type") or "") == "agent.model.manage":
                model_hint = str(
                    ((action or {}).get("payload") or {}).get("model")
                    or (action or {}).get("model")
                    or ""
                ).strip()
                if model_hint:
                    break
        if not model_hint and _looks_like_bare_model_token(text):
            model_hint = text
        prompt = (
            f"你刚发的是模型名「{model_hint}」。如果你是想切换模型，请明确说“切换到 {model_hint}”。如果你是在讨论这个模型，我就按聊天继续。"
            if model_hint
            else "我还不能确定你是想讨论模型，还是想切换模型。你直接说“切换到 xxx”我再执行。"
        )
        safe = dict(result)
        safe["reply"] = prompt
        safe["actions"] = []
        safe["notes"] = "agent_model_manage_intent_required"
        safe["requires_approval"] = False
        return safe
    if (
        not actions
        and _MODEL_EXECUTION_CLAIM_RE.search(str(result.get("reply") or ""))
        and _looks_like_bare_model_token(text)
        and not _has_explicit_model_manage_intent(text)
    ):
        safe = dict(result)
        safe["reply"] = (
            f"你刚发的是模型名「{text}」。如果你要我切换模型，请直接说“切换到 {text}”；不然我就默认你是在聊这个模型。"
        )
        safe["notes"] = "agent_model_reply_claim_blocked"
        return safe
    return result


_IMAGE_TRIGGER_PATTERNS = [
    # 名词式命令必须有空格或冒号分隔，避免把“生图的/生图，生视频需求”这类讨论误判成画图任务。
    _re.compile(
        r"^\s*(?:生图|画图|绘图|出图|做图|生成图片|生成图像)(?:\s+|[:：]\s*)(?P<prompt>.+)$",
        _re.I,
    ),
    # 兼容用户口语“画图画特朗普...”。
    _re.compile(r"^\s*(?:画图|绘图|出图|做图)(?P<prompt>画.+)$", _re.I),
    _re.compile(
        r"^\s*(?:帮我|请|麻烦|给我)?\s*(?:画|生成|绘制)\s*(?:[一1])?(?:张|个|只|幅|份)?\s*(?P<prompt>.+?)(?:图|图片|照片|壁纸)?\s*$",
        _re.I,
    ),
    _re.compile(
        r"^\s*(?:给我画|给我生成|给我来|帮我来|来|整)[一1]?(?:张|个|只|幅|份)?\s*(?P<prompt>.+?)(?:图|图片|照片|壁纸)?\s*$",
        _re.I,
    ),
    _re.compile(r"^\s*/?(?:draw|image|img|imagine|paint)\s+(?P<prompt>.+)$", _re.I),
]
_IMAGE_FALSE_COMMAND_PREFIXES = (
    "生图的",
    "画图的",
    "绘图的",
    "出图的",
    "做图的",
    "生成图片的",
    "生成图像的",
    "生图，",
    "生图,",
    "画图，",
    "画图,",
    "绘图，",
    "绘图,",
    "出图，",
    "出图,",
    "做图，",
    "做图,",
    "画出来",
    "画完",
    "画好",
    "画得",
    "画的",
    "生成出来",
    "生成的",
)
_IMAGE_FALSE_PROMPT_PREFIXES = (
    "的",
    "，",
    ",",
    "。",
    "、",
    "出来",
    "完",
    "好",
    "得",
    "的",
)
_IMAGE_STATUS_PATTERNS = [
    _re.compile(
        r"(画好了吗|画完了吗|出图了吗|生好了吗|好了吗|还没好吗|进度|结果呢|什么时候好|你画了吗)",
        _re.I,
    ),
]
_IMAGE_COOLDOWN = {}
_IMAGE_PENDING = {}
_IMAGE_CONFIG_CACHE = {"value": None, "expires_at": 0.0}
_IMAGE_CONFIG_LOCK = threading.Lock()


def _image_cooldown_key(platform, room_id, user_id):
    return f"{platform}:{room_id}:{user_id}"


def _image_cooldown_left(platform, room_id, user_id, cooldown_seconds=None):
    cooldown_seconds = (
        IMAGE_COOLDOWN_SECONDS
        if cooldown_seconds is None
        else max(0, int(cooldown_seconds))
    )
    if cooldown_seconds <= 0:
        return 0
    now = time.time()
    key = _image_cooldown_key(platform, room_id, user_id)
    until = float(_IMAGE_COOLDOWN.get(key, 0) or 0)
    if until <= now:
        _IMAGE_COOLDOWN.pop(key, None)
        return 0
    return int(until - now + 0.999)


def _set_image_cooldown(platform, room_id, user_id, cooldown_seconds=None):
    cooldown_seconds = (
        IMAGE_COOLDOWN_SECONDS
        if cooldown_seconds is None
        else max(0, int(cooldown_seconds))
    )
    if cooldown_seconds <= 0:
        return
    _IMAGE_COOLDOWN[_image_cooldown_key(platform, room_id, user_id)] = (
        time.time() + cooldown_seconds
    )


def _image_pending_key(platform, room_id, user_id):
    return f"{platform}:{room_id}:{user_id}"


def _get_image_pending(platform, room_id, user_id):
    item = _IMAGE_PENDING.get(_image_pending_key(platform, room_id, user_id))
    if not item:
        return None
    return dict(item)


def _set_image_pending(platform, room_id, user_id, prompt):
    _IMAGE_PENDING[_image_pending_key(platform, room_id, user_id)] = {
        "prompt": str(prompt or "")[:200],
        "started_at": time.time(),
    }


def _clear_image_pending(platform, room_id, user_id):
    _IMAGE_PENDING.pop(_image_pending_key(platform, room_id, user_id), None)


def detect_image_status_query(text):
    raw = str(text or "").strip()
    if not raw:
        return False
    return any(p.search(raw) for p in _IMAGE_STATUS_PATTERNS)


def detect_image_prompt(text):
    raw = str(text or "")
    raw = _re.sub(r"\[CQ:at,[^\]]+\]\s*", " ", raw)
    raw = _re.sub(r"@\S+\s*", " ", raw)
    raw = " ".join(raw.strip().split())
    if not raw:
        return ""
    compact = raw.replace(" ", "")
    if any(
        compact.lower().startswith(p.lower()) for p in _IMAGE_FALSE_COMMAND_PREFIXES
    ):
        return ""
    for pat in _IMAGE_TRIGGER_PATTERNS:
        m = pat.match(raw)
        if not m:
            continue
        prompt = " ".join(str(m.group("prompt") or "").split())
        if any(prompt.startswith(p) for p in _IMAGE_FALSE_PROMPT_PREFIXES):
            continue
        if len(prompt) >= 2 or any("\u4e00" <= ch <= "\u9fff" for ch in prompt):
            return prompt[:1200]
    return ""


def _get_chat_history(source, room_id, user_id, limit=12):
    """从 new-api 拉取该会话最近对话历史（按用户/群隔离）。失败返回空。"""
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL")
        or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
        or "http://127.0.0.1:3000"
    ).rstrip("/")
    secret = _chatops_secret()
    if not base or not secret:
        return []
    payload = {
        "source": source,
        "room_id": room_id or "",
        "user_external_id": str(user_id),
        "limit": limit,
    }
    try:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        req = urllib.request.Request(
            _chatops_url(base, "/api/agent/chatops/history/get", secret, source),
            data=data,
            headers=_chatops_headers(secret, source),
        )
        with urllib.request.urlopen(req, timeout=8) as resp:
            out = json.loads(resp.read().decode("utf-8") or "{}")
        d = out.get("data") if isinstance(out, dict) else None
        if isinstance(d, dict) and isinstance(d.get("history"), list):
            return d["history"]
    except Exception as e:
        print(
            f"[ChatHistory] get failed source={source} user={user_id}: {e}", flush=True
        )
    return []


def _append_chat_history(source, room_id, user_id, user_text, assistant_text):
    """把一轮对话存回 new-api（异步，不阻塞回复）。"""
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL")
        or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
        or "http://127.0.0.1:3000"
    ).rstrip("/")
    secret = _chatops_secret()
    if not base or not secret:
        return
    payload = {
        "source": source,
        "room_id": room_id or "",
        "user_external_id": str(user_id),
        "user_text": user_text or "",
        "assistant_text": assistant_text or "",
    }

    def _do():
        try:
            data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            req = urllib.request.Request(
                _chatops_url(base, "/api/agent/chatops/history/append", secret, source),
                data=data,
                headers=_chatops_headers(secret, source),
            )
            with urllib.request.urlopen(req, timeout=8) as resp:
                resp.read(2048)
        except Exception as e:
            print(
                f"[ChatHistory] append failed source={source} user={user_id}: {e}",
                flush=True,
            )

    _submit_background(_do, task_name="chat_history_append")


def _capture_chatops_message(
    source,
    room_id,
    user_id,
    username,
    text,
    message_id="",
    user_role="",
    is_admin=False,
    raw=None,
):
    """只沉淀记忆，不触发 Agent 回复；用于 adapter 直接调模型/忽略水聊时的后台记忆。"""
    _remember_recent_chat_message(
        source,
        room_id,
        user_id,
        username,
        text,
        message_id=message_id,
        user_role=user_role,
        is_admin=is_admin,
        raw=raw,
    )
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL")
        or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
        or "http://127.0.0.1:3000"
    ).rstrip("/")
    secret = _chatops_secret()
    if not base or not secret or not text:
        return
    payload = {
        "source": source,
        "room_id": room_id or "",
        "message_id": str(message_id or ""),
        "user_external_id": str(user_id or ""),
        "username": username or str(user_id or ""),
        "user_role": user_role or "",
        "is_admin": bool(is_admin),
        "text": text or "",
        "raw": raw or {},
    }

    def _do():
        try:
            data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            req = urllib.request.Request(
                _chatops_url(base, "/api/agent/chatops/capture", secret, source),
                data=data,
                headers=_chatops_headers(secret, source),
            )
            with urllib.request.urlopen(req, timeout=8) as resp:
                resp.read(2048)
        except Exception as e:
            print(
                f"[ChatMemory] capture failed source={source} user={user_id}: {e}",
                flush=True,
            )

    _submit_background(_do, task_name="chat_memory_capture")


def _qq_target_meta(raw, msg):
    meta = {}
    raw_text = str(raw.get("raw_message") or raw.get("message") or msg or "")
    m = _re.search(r"\[CQ:reply,id=(-?\d+)\]", raw_text)
    if m:
        meta["target_message_id"] = m.group(1)
        meta["reply_message_id"] = m.group(1)
    m = _re.search(r"\[CQ:at,qq=(\d{5,20})\]", raw_text)
    if m:
        meta["target_external_id"] = m.group(1)
        meta["target_user_id"] = m.group(1)
    return meta


def _tg_display_name(user):
    user = user or {}
    return _first_nonempty(
        user.get("username"),
        " ".join(
            [
                str(user.get("first_name") or "").strip(),
                str(user.get("last_name") or "").strip(),
            ]
        ).strip(),
        user.get("id"),
    )


def _tg_get_member_role(chat_id, user_id):
    if not TG_BOT_TOKEN or not chat_id or not user_id:
        return "", False
    try:
        api = f"https://api.telegram.org/bot{TG_BOT_TOKEN}/getChatMember"
        body = json.dumps(
            {
                "chat_id": int(chat_id)
                if str(chat_id).lstrip("-").isdigit()
                else chat_id,
                "user_id": int(user_id),
            },
            ensure_ascii=False,
        ).encode("utf-8")
        req = urllib.request.Request(
            api, data=body, headers={"Content-Type": "application/json"}
        )
        with urllib.request.urlopen(req, timeout=8) as r:
            data = json.loads(r.read(65536).decode("utf-8") or "{}")
        status = str(((data.get("result") or {}).get("status")) or "").lower()
        return status, status in ("administrator", "creator")
    except Exception as e:
        print(
            f"[Telegram] getChatMember failed chat={chat_id} user={user_id}: {e}",
            flush=True,
        )
        return "", False


def _tg_target_meta(msg):
    meta = {}
    reply = msg.get("reply_to_message") or {}
    if reply:
        if reply.get("message_id") is not None:
            meta["target_message_id"] = str(reply.get("message_id"))
            meta["reply_message_id"] = str(reply.get("message_id"))
        user = reply.get("from") or {}
        if user.get("id") is not None:
            meta["target_external_id"] = str(user.get("id"))
            meta["target_user_id"] = str(user.get("id"))
    return meta


_QQ_EXPLICIT_GROUP_RE = _re.compile(r"(?:qq群?|群号)\D{0,6}(\d{5,20})", _re.I)
_TG_EXPLICIT_GROUP_RE = _re.compile(
    r"(?:tg群?|telegram群?|chat|群号|群)\D{0,6}(-100\d{5,20}|-?\d{5,20})", _re.I
)


def _extract_explicit_room_id(source, text):
    source = str(source or "").lower().strip()
    text = str(text or "")
    if source == "qq":
        m = _QQ_EXPLICIT_GROUP_RE.search(text)
        return str(m.group(1) or "").strip() if m else ""
    if source in ("tg", "telegram"):
        m = _TG_EXPLICIT_GROUP_RE.search(text)
        return str(m.group(1) or "").strip() if m else ""
    return ""


def _resolve_effective_room_context(
    source, msg_type, current_room_id, user_id, username, text, is_admin=False
):
    current_room_id = str(current_room_id or "").strip()
    if current_room_id:
        return current_room_id, "current"
    explicit_room_id = _extract_explicit_room_id(source, text)
    if explicit_room_id:
        return explicit_room_id, "explicit"
    if str(msg_type or "").lower().strip() != "private" or not is_admin:
        return "", ""
    if _is_ad_lookup_followup(text):
        room_id = _resolve_ad_lookup_context(source, user_id)
        if room_id:
            return room_id, "ad_followup"
    room_id = _resolve_recent_user_room(source, user_id)
    if room_id:
        return room_id, "recent"
    return "", ""


def _action_user_summaries(actions, results):
    summaries = []
    for action, result in zip(actions or [], results or []):
        s = _action_observe_summary(action, result)
        if s:
            summaries.append(s)
    return summaries


# ===== 群消息风控：规则粗筛 + LLM 复核，确认违规则撤回+提醒（群主/管理员豁免）=====
def _load_system_config():
    try:
        with open(GAME_CONFIG_PATH, encoding="utf-8") as _f:
            return json.load(_f).get("system", {})
    except Exception:
        return {}


_SYS_CFG = _load_system_config()
RISK_ENABLED = _SYS_CFG.get("group_risk_enabled", True)


def _burn_after_seconds():
    """动态读 burn_after_seconds(配置改后即时生效,不依赖模块级 _SYS_CFG)。"""
    try:
        return int(_load_system_config().get("burn_after_seconds", 15))
    except Exception:
        return 15


# 规则粗筛关键词：命中才进 LLM 复核（降成本、降误判）。擦边/玩笑不在此列。
_RISK_PRESCREEN_PATTERNS = [
    # 交易/引流
    "收u",
    "出u",
    "收U",
    "出U",
    "换u",
    "换U",
    "usdt",
    "泰达",
    "出号",
    "收号",
    "卖号",
    "买号",
    "加微信",
    "加vx",
    "加v信",
    "加薇",
    "私聊我",
    "加我私",
    "低价出",
    "代充",
    "招代理",
    "发卡网",
    # 赌博/诈骗
    "博彩",
    "赌博",
    "彩票群",
    "稳赚",
    "包赢",
    "日入",
    "躺赚",
    "刷单",
    "兼职日结",
    "点击链接领",
    # 色情低俗
    "约炮",
    "裸聊",
    "黄片",
    "激情",
    "一夜情",
    "色情",
    # 政治敏感（粗筛，最终由LLM判）
    "翻墙",
    "fanqiang",
]


def _risk_prescreen(text):
    t = (text or "").lower()
    if not t.strip():
        return False
    for p in _RISK_PRESCREEN_PATTERNS:
        if p.lower() in t:
            return True
    # 含外链 + 引流意图的组合（http + 加群/私聊）
    if ("http://" in t or "https://" in t or ".com" in t or ".cn" in t) and any(
        k in t for k in ("加群", "私聊", "领取", "免费送", "点击")
    ):
        return True
    return False


def _risk_llm_confirm(text):
    """LLM 复核：判断是否真违规。返回 (is_violation: bool, category: str)。
    只拦真正敏感：交易引流/赌博诈骗/色情低俗/政治敏感/违法/人身攻击。擦边玩笑/正常闲聊不算。"""
    if not API_BASE_URL or not API_KEY:
        return False, ""
    sys_p = (
        "你是群聊内容安全审核员。判断给定消息是否属于需要撤回的违规内容。"
        "违规类别仅限：交易引流(收U/出号/卖号/加微信私下交易/代充招代理)、赌博诈骗(博彩/刷单/稳赚兼职)、"
        "色情低俗、政治敏感、违法犯罪、严重人身攻击辱骂、垃圾广告导流。"
        "注意：正常聊天、玩笑、玩梗、角色扮演、擦边调侃、吐槽、提问、技术讨论都【不算】违规。"
        '只返回严格JSON：{"violation":true/false,"category":"类别或空"}。'
    )
    body = {
        "model": MODEL,
        "messages": [
            {"role": "system", "content": sys_p},
            {"role": "user", "content": "待审核消息：\n" + str(text)[:500]},
        ],
        "temperature": 0,
        "stream": False,
    }
    try:
        req = urllib.request.Request(
            API_BASE_URL + "/chat/completions",
            json.dumps(body).encode("utf-8"),
            {"Authorization": "Bearer " + API_KEY, "Content-Type": "application/json"},
        )
        with urllib.request.urlopen(req, timeout=12) as resp:
            data = json.loads(resp.read().decode("utf-8"))
        content = (
            ((data.get("choices") or [{}])[0].get("message") or {}).get("content")
        ) or ""
        parsed = extract_json(content)
        return bool(parsed.get("violation")), str(parsed.get("category") or "")
    except Exception as e:
        print(f"[Risk] llm confirm failed: {e}", flush=True)
        return False, ""


def handle_group_risk(raw, gid, uid, msg, mid):
    """对群消息做风控。返回 True 表示已判定违规并处置（调用方应停止后续处理）。"""
    if not RISK_ENABLED or not gid or not msg:
        return False
    # 豁免仅基于配置名单或实时 OneBot 成员查询，不能信任 webhook sender.role。
    username = raw.get("sender", {}).get("card") or raw.get("sender", {}).get(
        "nickname", uid
    )
    trusted_admin, _ = _trusted_qq_admin(gid, uid)
    if _is_source_admin("qq", uid, username) or trusted_admin:
        return False
    # 规则粗筛
    if not _risk_prescreen(msg):
        return False
    # LLM 复核
    is_viol, category = _risk_llm_confirm(msg)
    if not is_viol:
        return False
    print(
        f"[Risk] violation group={gid} user={uid} category={category} msg={msg[:50]}",
        flush=True,
    )
    # 处置：撤回违规消息 + 发提醒（提醒也阅后即焚，保持清爽）
    if mid:
        onebot_delete_msg(mid)
    tip = f"⚠️ 检测到疑似违规内容（{category or '违规'}），已自动撤回。请勿在群内发布交易引流、广告、赌博诈骗、色情或敏感信息，谢谢配合～"
    tip_mid = send_qq_reply("group", gid, tip)
    schedule_burn([tip_mid], delay=20)
    return True


def confirm_bind_via_newapi(source, room_id, user_id, username, code):
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL")
        or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
        or "http://127.0.0.1:3000"
    ).rstrip("/")
    secret = _chatops_secret()
    if not base or not secret:
        return {"ok": False, "message": "chatops secret not configured"}
    room_id = (room_id or "").strip()
    if not room_id:
        return {
            "ok": False,
            "message": "当前绑定必须在群聊内发起，请在目标群内发送“绑定 绑定码”后重试。",
        }
    payload = {
        "source": source,
        "room_id": room_id,
        "scope": "room",
        "user_external_id": str(user_id),
        "username": username or str(user_id),
        "code": code,
    }
    try:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        req = urllib.request.Request(
            _chatops_url(base, "/api/agent/chatops/bind-confirm", secret, source),
            data=data,
            headers=_chatops_headers(secret, source),
        )
        with urllib.request.urlopen(req, timeout=10) as resp:
            out = json.loads(resp.read().decode("utf-8") or "{}")
        ident = _unwrap_newapi_response(out)
        if ident and (
            ident.get("user_bound")
            or ident.get("bound")
            or ident.get("new_api_user_id")
        ):
            for k in [
                _identity_cache_key(source, room_id, user_id),
                _identity_cache_key(source, "", user_id),
            ]:
                _IDENTITY_CACHE.pop(k, None)
            ident["ok"] = True
            return ident
        return {
            "ok": False,
            "message": out.get("message") or out.get("error") or "绑定失败",
        }
    except Exception as e:
        print(f"[Bind] confirm failed source={source} user={user_id}: {e}", flush=True)
        return {"ok": False, "message": str(e)}


def resolve_identity_via_newapi(source, room_id, user_id, username):
    # Use the local New API origin for identity resolution.  Going through
    # PUBLIC_BASE_URL may traverse Cloudflare / public reverse proxies and
    # trigger 403/429 during group-game bursts.  Keep PUBLIC_BASE_URL only
    # as a last fallback so display URLs remain unchanged elsewhere.
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL")
        or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
        or "http://127.0.0.1:3000"
        or os.environ.get("PUBLIC_BASE_URL", "")
    ).rstrip("/")
    secret = _chatops_secret()
    if not base or not secret:
        return {}
    if not user_id:
        return {}
    ck = _identity_cache_key(source, room_id, user_id)
    now = time.time()
    cached = _IDENTITY_CACHE.get(ck)
    if cached and now - cached[0] < IDENTITY_CACHE_TTL:
        return dict(cached[1])
    payload = {
        "source": source,
        "room_id": room_id or "",
        "user_external_id": str(user_id),
        "username": username or str(user_id),
    }
    try:
        data = json.dumps(payload).encode("utf-8")
        req = urllib.request.Request(
            _chatops_url(base, "/api/agent/chatops/resolve", secret, source),
            data=data,
            headers=_chatops_headers(secret, source),
        )
        with urllib.request.urlopen(req, timeout=8) as resp:
            if resp.status != 200:
                return {}
            out = json.loads(resp.read().decode("utf-8") or "{}")
            ident = _unwrap_newapi_response(out)
            if ident:
                _IDENTITY_CACHE[ck] = (now, ident)
                return dict(ident)
    except Exception as e:
        stale = _IDENTITY_CACHE.get(ck)
        if stale:
            print(
                f"[Identity] resolve failed; using stale cache source={source} user={user_id}: {e}",
                flush=True,
            )
            return dict(stale[1])
        print(
            f"[Identity] resolve failed source={source} user={user_id}: {e}", flush=True
        )
    return {}


def run_game_actions_via_newapi(
    actions,
    source,
    room_id,
    external_user_id,
    username="",
    user_role="",
    is_admin=False,
    message_id="",
    raw=None,
):
    if not actions:
        return []
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL") or "http://127.0.0.1:3000"
    ).rstrip("/")
    secret = _chatops_secret()
    out = []
    for action in actions:
        try:
            payload = {
                "source": source,
                "room_id": room_id or "",
                "message_id": str(message_id or ""),
                "user_external_id": str(external_user_id or ""),
                "username": username or str(external_user_id or ""),
                "user_role": user_role or "",
                "is_admin": bool(is_admin),
                "raw": raw or {},
                "action": action,
            }
            data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            req = urllib.request.Request(
                _chatops_url(base, "/api/agent/chatops/action", secret, source),
                data=data,
                headers=_chatops_headers(secret, source),
            )
            with urllib.request.urlopen(req, timeout=15) as r:
                out.append(json.loads(r.read().decode("utf-8") or "{}"))
        except Exception as e:
            print(
                f"[Action] execute failed action={action.get('type')}: {e}", flush=True
            )
            out.append({"ok": False, "error": str(e), "type": action.get("type")})
    return out


_KNOWN_GAME_CODES = (
    "banker_guess",
    "duel_compare",
    "duel_idiom",
    "duel_rps",
    "redpacket",
    "treasure",
    "lottery",
    "predict",
    "bounty",
    "dice",
    "wheel",
    "fortune",
    "quiz",
    "checkin",
    "verify",
    "invite",
    "leaderboard",
    "luckybag",
    "profile",
    "menu",
)


def _infer_game_code(action):
    action = action or {}
    explicit = str(action.get("game_code") or action.get("game") or "").strip().lower()
    if explicit:
        return explicit.replace(" ", "_")[:64]
    reason = str(action.get("reason") or action.get("event") or "").strip().lower()
    for code in _KNOWN_GAME_CODES:
        if reason.startswith(code):
            return code
    if reason:
        return reason.split("_", 1)[0][:64]
    return "game"


def _game_budget():
    try:
        return getattr(getattr(_director, "director", None), "_budget", None)
    except Exception:
        return None


def _peek_game_commission_events(source, actions):
    budget = _game_budget()
    if not budget or not hasattr(budget, "peek_income_events"):
        return 0, []
    pools = sorted(
        {str((a or {}).get("budget_pool") or "game") for a in (actions or [])}
    ) or ["game"]
    events = []
    thread_id = threading.get_ident()
    source = source or "default"
    for pool in pools:
        try:
            events.extend(
                budget.peek_income_events(
                    pool=pool, platform=source, thread_id=thread_id, max_age=120
                )
            )
        except Exception as e:
            print(f"[Settlement] peek commission failed pool={pool}: {e}", flush=True)
    seen = {}
    for e in events:
        try:
            seq = int(e.get("seq", 0) or 0)
        except Exception:
            seq = 0
        if seq:
            seen[seq] = e
    out = [seen[k] for k in sorted(seen)]
    total = sum(max(0, int(e.get("amount", 0) or 0)) for e in out)
    return total, out


def _commit_game_commission_events(events):
    if not events:
        return 0
    budget = _game_budget()
    if not budget or not hasattr(budget, "commit_income_events"):
        return 0
    return budget.commit_income_events([e.get("seq") for e in events])


def _settlement_event_material(actions, event_key=""):
    event_key = str(event_key or "").strip()
    if event_key:
        return event_key
    explicit = []
    for action in actions or []:
        if not isinstance(action, dict):
            continue
        for key in (
            "event_key",
            "idempotency_key",
            "grant_idempotency_key",
            "round_key",
            "event_id",
        ):
            value = str(action.get(key) or "").strip()
            if value:
                explicit.append(f"{key}:{value}")
    if explicit:
        return "|".join(explicit)
    return json.dumps(
        actions or [],
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
        default=str,
    )


def _stable_settlement_id(
    actions, source, room_id, external_user_id, game_code, event_key=""
):
    material = {
        "source": str(source or "").strip().lower(),
        "room_id": str(room_id or "").strip(),
        "external_user_id": str(external_user_id or "").strip(),
        "game_code": str(game_code or "game").strip().lower(),
        "event_key": _settlement_event_material(actions, event_key),
    }
    digest = hashlib.sha256(
        json.dumps(
            material, ensure_ascii=False, sort_keys=True, separators=(",", ":")
        ).encode("utf-8")
    ).hexdigest()[:24]
    return f"{material['source']}-{material['game_code'][:18]}-{digest}"


def _tag_settlement_result(result, settlement_id):
    tagged = (
        dict(result)
        if isinstance(result, dict)
        else {"ok": False, "error": "invalid settlement response"}
    )
    tagged.setdefault("action_type", "reward.settlement.batch")
    tagged.setdefault("settlement_id", settlement_id)
    tagged.setdefault("idempotency_key", settlement_id)
    return tagged


def run_game_settlement(actions, source, room_id, external_user_id, event_key=""):
    """批量原子结算: 多 mutation 一次 POST reward.settlement.batch；附带手续费/奖池审计元数据。"""
    if not actions:
        return []
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL") or "http://127.0.0.1:3000"
    ).rstrip("/")
    secret = _chatops_secret()
    source = str(source or "unknown").lower()
    if source == "telegram":
        source = "tg"
    game_code = _infer_game_code(actions[0] if actions else {})
    commission_quota, commission_events = _peek_game_commission_events(source, actions)
    mutations = []
    primary_user_id = 0
    primary_external_id = ""
    for a in actions:
        delta = int(a.get("quota_amount", 0) or 0)
        if delta == 0:
            continue
        stype = a.get("source_type") or ("game_payout" if delta > 0 else "game_stake")
        uid = a.get("user_id") or a.get("target_id") or 0
        target_external_id = str(
            a.get("target_external_id")
            or a.get("external_user_id")
            or a.get("user_external_id")
            or ""
        )
        try:
            if primary_user_id <= 0 and int(uid or 0) > 0:
                primary_user_id = int(uid or 0)
        except Exception:
            pass
        if not primary_external_id and target_external_id:
            primary_external_id = target_external_id
        reason = str(a.get("reason", "") or "")
        action_game = _infer_game_code(a)
        if action_game != "game":
            game_code = action_game
        meta = {
            "source": source,
            "platform": source,
            "room_id": room_id,
            "game": action_game,
            "game_code": action_game,
            "reason": reason,
            "target_type": a.get("target_type", "user"),
            "target_external_id": target_external_id,
            "quota_amount": delta,
            "budget_pool": a.get("budget_pool", "game"),
        }
        mutations.append(
            {
                "user_id": int(uid),
                "delta": delta,
                "pool_type": a.get("budget_pool", "game"),
                "source_type": stype,
                "remark": reason,
                "game_code": action_game,
                "metadata_json": json.dumps(meta, ensure_ascii=False),
            }
        )
    if not mutations:
        _commit_game_commission_events(commission_events)
        return []
    sid = _stable_settlement_id(
        actions, source, room_id, external_user_id, game_code, event_key
    )
    payload = {
        "source": source,
        "platform": source,
        "room_id": room_id or "",
        "user_external_id": str(external_user_id or ""),
        "action": {
            "type": "reward.settlement.batch",
            "idempotency_key": sid,
            "settlement_id": sid,
            "round_key": sid,
            "game_code": game_code,
            "new_api_user_id": primary_user_id,
            "target_external_id": primary_external_id,
            "commission_quota": commission_quota,
            "commission_events": commission_events,
            "mutations": mutations,
        },
    }
    try:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        req = urllib.request.Request(
            _chatops_url(base, "/api/agent/chatops/action", secret, source),
            data=data,
            headers=_chatops_headers(secret, source),
        )
        with urllib.request.urlopen(req, timeout=15) as r:
            resp = json.loads(r.read().decode("utf-8") or "{}")
        _commit_game_commission_events(commission_events)
        return [_tag_settlement_result(resp, sid)]
    except Exception as e:
        # 结算失败时也清理本轮已记录的临时手续费事件，避免串到下一轮。
        _commit_game_commission_events(commission_events)
        print(f"[Settlement] batch failed sid={sid}: {e}", flush=True)
        return [_tag_settlement_result({"ok": False, "error": str(e)}, sid)]


def _is_settlement_compatible_action(action):
    if (action or {}).get("type") != "reward.grant.small":
        return False
    try:
        return int((action or {}).get("quota_amount", 0) or 0) != 0
    except Exception:
        return False


def _newapi_action_data(result):
    if not isinstance(result, dict):
        return {}
    if isinstance(result.get("data"), dict):
        return dict(result.get("data") or {})
    return dict(result)


def _newapi_action_summary(result):
    if not isinstance(result, dict):
        return False, "", "", ""
    data = _newapi_action_data(result)
    raw = data.get("result_json") or result.get("result_json") or ""
    parsed = {}
    if isinstance(raw, str) and raw:
        try:
            parsed = json.loads(raw)
        except Exception:
            parsed = {}
    elif isinstance(raw, dict):
        parsed = raw
    summary = str(
        parsed.get("summary") or data.get("summary") or result.get("summary") or ""
    ).strip()
    error = str(
        parsed.get("error") or data.get("error") or result.get("error") or ""
    ).strip()
    status = str(data.get("status") or result.get("status") or "").strip()
    ok = bool(result.get("success") or result.get("ok") or status == "completed")
    return ok, summary, error, status


def _action_observe_summary(action, result):
    t = str((action or {}).get("type") or "")
    ok, summary, err, status = _newapi_action_summary(result)
    summary = summary or err
    if not summary:
        return ""
    if t.startswith("group."):
        return summary if ok else "群管理没处理成：" + summary
    if t == "fund.report.read":
        return "运营基金：" + summary
    if t == "site.logs.read":
        return "站点日志：" + summary
    if t == "chatops.logs.read":
        return "群聊运营日志：" + summary
    if t == "budget.check":
        return "预算日控：" + summary
    if t in (
        "user.quota.read",
        "agent.model.manage",
        "agent.skill.install",
        "agent.skill.list",
    ):
        return summary
    return ""


def _is_settlement_action_result(result):
    if not isinstance(result, dict):
        return False
    data = _newapi_action_data(result)
    action_type = str(
        result.get("action_type")
        or result.get("type")
        or data.get("action_type")
        or data.get("type")
        or ""
    ).strip()
    if action_type == "reward.settlement.batch":
        return True
    return bool(result.get("settlement_id") or data.get("settlement_id"))


def _action_pipeline_user_summaries(action_results):
    # Return short user-facing settlement summaries for game quota mutations.
    summaries = []
    for result in action_results or []:
        if not _is_settlement_action_result(result):
            continue
        ok, parsed_summary, parsed_error, _ = _newapi_action_summary(result)
        settlement_id = str(
            result.get("settlement_id") or result.get("idempotency_key") or ""
        )
        summary = str(parsed_summary or "").strip()
        err = str(
            parsed_error or result.get("message") or result.get("error") or ""
        ).strip()
        if ok and summary:
            summaries.append("💳 额度结算：" + summary)
        elif ok:
            suffix = f"（流水 {settlement_id[-8:]}）" if settlement_id else ""
            summaries.append("💳 额度结算：已完成" + suffix)
        else:
            suffix = f"（流水 {settlement_id[-8:]}）" if settlement_id else ""
            summaries.append(
                "⚠️ 额度结算失败：" + (err or "上游未返回明确原因") + suffix
            )
    return summaries


def _finalize_game_reply(result, action_results):
    action_results = list(action_results or [])
    settlement_results = [
        row for row in action_results if _is_settlement_action_result(row)
    ]
    failed = [row for row in settlement_results if not _newapi_action_summary(row)[0]]
    safe = dict(result or {})
    summaries = _action_pipeline_user_summaries(action_results)
    if failed:
        # Never retain optimistic payout/refund language when settlement failed.
        safe["reply"] = (
            "本轮游戏结果已生成，但额度结算失败，本次奖励/扣款均不按成功处理。\n"
            + "\n".join(summaries)
        )
        safe["notes"] = "game_settlement_failed"
    elif summaries:
        safe["reply"] = (
            str(safe.get("reply") or "").rstrip() + "\n\n" + "\n".join(summaries)
        )
    if action_results:
        safe["action_results"] = action_results
    return safe


def run_game_action_pipeline(actions, source, room_id, external_user_id, event_key=""):
    if not actions:
        return {"results": [], "summaries": []}
    settlement_actions = []
    passthrough_actions = []
    for action in actions:
        if _is_settlement_compatible_action(action):
            settlement_actions.append(action)
        else:
            passthrough_actions.append(action)
    results = []
    summaries = []
    if settlement_actions:
        results.extend(
            run_game_settlement(
                settlement_actions,
                source,
                room_id,
                external_user_id,
                event_key=event_key,
            )
        )
    if passthrough_actions:
        pass_results = run_game_actions_via_newapi(
            passthrough_actions, source, room_id, external_user_id
        )
        tagged_pass_results = []
        for action, result in zip(passthrough_actions, pass_results):
            tagged = (
                dict(result)
                if isinstance(result, dict)
                else {"ok": False, "error": "invalid action response"}
            )
            tagged.setdefault("action_type", str((action or {}).get("type") or ""))
            tagged_pass_results.append(tagged)
            summary = _action_observe_summary(action, tagged)
            if summary:
                summaries.append(summary)
        results.extend(tagged_pass_results)
    return {"results": results, "summaries": summaries}


if _HAS_GAME_DIRECTOR:
    try:
        _director.set_action_fn(run_game_action_pipeline)
    except Exception as e:
        print(f"[GameDirector] set action callback failed: {e}", flush=True)

# 阅后即焚：burn_after_seconds=0 可关闭（配置在 game_config.json → system）
BURN_AFTER_SECONDS = int(_SYS_CFG.get("burn_after_seconds", 15))

# 撤回任务使用独立的单线程定时队列。延迟等待不占用通用异步 worker，
# 队列有明确上限，避免消息洪峰时无限创建 Timer/Thread 或堆积 Future。
# 普通任务满载时拒绝入队；绑定码等敏感消息由调用方同步撤回。
_BURN_QUEUE_CAPACITY = max(1, int(os.environ.get("HERMES_BURN_QUEUE_CAPACITY", "512")))
_BURN_QUEUE = []
_BURN_QUEUE_CONDITION = threading.Condition()
_BURN_QUEUE_SEQUENCE = 0
_BURN_SCHEDULER_GENERATION = 0
_BURN_WORKER = None


def onebot_delete_msg(message_id):
    """撤回一条消息（bot 自己的或用户的，撤用户消息需 bot 为群管理员）。"""
    if not message_id or not ONEBOT_URL:
        return False
    try:
        body = json.dumps({"message_id": int(message_id)}).encode("utf-8")
        req = urllib.request.Request(
            f"{ONEBOT_URL}/delete_msg",
            data=body,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {ONEBOT_TOKEN}",
            },
        )
        with urllib.request.urlopen(req, timeout=10) as r:
            r.read(4096)
        return True
    except Exception as e:
        print(f"[Burn] delete_msg failed id={message_id}: {e}", flush=True)
        return False


def _burn_scheduler_worker(generation):
    global _BURN_WORKER
    try:
        while True:
            with _BURN_QUEUE_CONDITION:
                while generation == _BURN_SCHEDULER_GENERATION and not _BURN_QUEUE:
                    _BURN_QUEUE_CONDITION.wait()
                if generation != _BURN_SCHEDULER_GENERATION:
                    return
                due_at, _, message_ids = _BURN_QUEUE[0]
                remaining = due_at - time.monotonic()
                if remaining > 0:
                    _BURN_QUEUE_CONDITION.wait(timeout=remaining)
                    continue
                _heapq.heappop(_BURN_QUEUE)
            for message_id in message_ids:
                try:
                    onebot_delete_msg(message_id)
                except Exception as exc:
                    log(
                        {
                            "event": "burn_delete_error",
                            "message_id": str(message_id),
                            "error": str(exc)[:300],
                        }
                    )
    finally:
        with _BURN_QUEUE_CONDITION:
            if _BURN_WORKER is threading.current_thread():
                _BURN_WORKER = None


def _ensure_burn_scheduler_locked():
    global _BURN_WORKER
    if _BURN_WORKER is not None and _BURN_WORKER.is_alive():
        return _BURN_WORKER
    generation = _BURN_SCHEDULER_GENERATION
    worker = threading.Thread(
        target=_burn_scheduler_worker,
        args=(generation,),
        name="hermes-burn-scheduler",
        daemon=True,
    )
    _BURN_WORKER = worker
    worker.start()
    return worker


def _burn_scheduler_snapshot():
    with _BURN_QUEUE_CONDITION:
        worker = _BURN_WORKER
        return {
            "queued": len(_BURN_QUEUE),
            "capacity": _BURN_QUEUE_CAPACITY,
            "worker_alive": bool(worker and worker.is_alive()),
            "worker_name": worker.name if worker else "",
        }


def _stop_burn_scheduler(clear_pending=False, join_timeout=1.0):
    """停止当前撤回调度线程；服务关闭时保留队列已无意义，可选择清空。"""
    global _BURN_SCHEDULER_GENERATION, _BURN_WORKER
    with _BURN_QUEUE_CONDITION:
        old_worker = _BURN_WORKER
        _BURN_SCHEDULER_GENERATION += 1
        _BURN_WORKER = None
        if clear_pending:
            _BURN_QUEUE.clear()
        _BURN_QUEUE_CONDITION.notify_all()
    if old_worker and old_worker is not threading.current_thread():
        old_worker.join(timeout=max(0.0, float(join_timeout)))


def _reset_burn_scheduler_for_tests(capacity=None, start_worker=False):
    """测试隔离钩子：废弃旧代次、清空队列，并可调整本模块实例容量。"""
    global _BURN_QUEUE_CAPACITY, _BURN_QUEUE_SEQUENCE
    _stop_burn_scheduler(clear_pending=True, join_timeout=0.5)
    with _BURN_QUEUE_CONDITION:
        _BURN_QUEUE_SEQUENCE = 0
        if capacity is not None:
            _BURN_QUEUE_CAPACITY = max(1, int(capacity))
    if start_worker:
        with _BURN_QUEUE_CONDITION:
            _ensure_burn_scheduler_locked()


def _normalize_burn_message_ids(message_ids):
    raw_ids = message_ids if isinstance(message_ids, (list, tuple)) else [message_ids]
    ids = []
    seen_ids = set()
    for message_id in raw_ids:
        if not message_id:
            continue
        normalized = str(message_id).strip()
        if not normalized or normalized in seen_ids:
            continue
        seen_ids.add(normalized)
        ids.append(normalized)
    return ids


def schedule_burn(message_ids, delay=None):
    """把撤回任务放入单一有界定时队列；成功入队返回 True。"""
    global _BURN_QUEUE_SEQUENCE
    if delay is None:
        delay = _burn_after_seconds()
    try:
        delay = float(delay)
    except (TypeError, ValueError):
        return False
    ids = _normalize_burn_message_ids(message_ids)
    if delay <= 0 or not ids:
        return False
    due_at = time.monotonic() + delay
    with _BURN_QUEUE_CONDITION:
        if len(_BURN_QUEUE) >= _BURN_QUEUE_CAPACITY:
            accepted = False
        else:
            _BURN_QUEUE_SEQUENCE += 1
            _heapq.heappush(_BURN_QUEUE, (due_at, _BURN_QUEUE_SEQUENCE, tuple(ids)))
            _ensure_burn_scheduler_locked()
            _BURN_QUEUE_CONDITION.notify()
            accepted = True
    if not accepted:
        log(
            {
                "event": "burn_queue_overload",
                "queued": _BURN_QUEUE_CAPACITY,
                "capacity": _BURN_QUEUE_CAPACITY,
                "message_count": len(ids),
            }
        )
    return accepted


def _schedule_sensitive_burn(message_ids, delay=None):
    ids = _normalize_burn_message_ids(message_ids)
    if not ids:
        return False
    if schedule_burn(ids, delay=delay):
        return True
    results = []
    for message_id in ids:
        try:
            deleted = bool(onebot_delete_msg(message_id))
        except Exception as exc:
            deleted = False
            log(
                {
                    "event": "sensitive_burn_sync_error",
                    "message_id": message_id,
                    "error": str(exc)[:300],
                }
            )
        results.append({"message_id": message_id, "deleted": deleted})
    log(
        {
            "event": "sensitive_burn_sync_fallback",
            "message_ids": ids,
            "deleted_ids": [item["message_id"] for item in results if item["deleted"]],
            "failed_ids": [
                item["message_id"] for item in results if not item["deleted"]
            ],
        }
    )
    return all(item["deleted"] for item in results)


# ============================================================================
# 输出脱敏（安全边界第③层）：屏蔽密钥/令牌/内部地址/路径。
# 配置在 game_config.json → system.sanitize_enabled / sanitize_extra_patterns
# ============================================================================

_SENSITIVE_PATTERNS = [
    # API key / 令牌：sk-、chatops_、Bearer、长随机串
    _re.compile(r"sk-[A-Za-z0-9_\-]{16,}"),
    _re.compile(r"chatops_[A-Za-z0-9_\-]{8,}"),
    _re.compile(r"(?i)\bBearer\s+[A-Za-z0-9._\-]{12,}"),
    _re.compile(
        r"(?i)\b(?:api[_\-]?key|secret|token|password|passwd|pwd)\s*[:=]\s*\S{6,}"
    ),
    # OneBot / webhook secret 这类 40+ 长 base62 串
    _re.compile(r"\b[A-Za-z0-9]{40,}\b"),
    # JWT
    _re.compile(r"\beyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}"),
    # 内网/回环地址与端口
    _re.compile(
        r"\b(?:127\.0\.0\.1|0\.0\.0\.0|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3})(?::\d{2,5})?\b"
    ),
    # 公网 IPv4（兜底，避免把站点内部 IP 暴露给群）
    _re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b"),
    # 内部文件路径
    _re.compile(r"(?:/opt|/var|/etc|/root|/usr/local)/[\w./\-]+"),
    # 邮箱（避免管理员邮箱外泄）
    _re.compile(r"\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b"),
]

# 站点自定义追加模式（环境变量，||| 分隔）
for _p in (
    _SYS_CFG.get("sanitize_extra_patterns", [])
    if isinstance(_SYS_CFG.get("sanitize_extra_patterns", []), list)
    else []
):
    _p = _p.strip()
    if _p:
        try:
            _SENSITIVE_PATTERNS.append(_re.compile(_p))
        except Exception as _e:
            print(f"[Sanitize] bad extra pattern {_p!r}: {_e}", flush=True)

# 敏感关键词：命中则在内部审计日志留痕（不改变文本，仅用于发现「试探/泄露」趋势）
_SENSITIVE_HINT = _re.compile(
    r"(?i)(风控规则|清零额度|reset_quota|管理员密码|root密码|webhook.?secret|onebot.?token)"
)


def _sanitize_reply(text):
    """屏蔽发给用户文本里的密钥/令牌/内部地址/路径。返回脱敏后的文本。"""
    if not text:
        return text
    s = str(text)
    if not _SYS_CFG.get("sanitize_enabled", True):
        return s
    hit = 0
    for pat in _SENSITIVE_PATTERNS:
        s, n = pat.subn("***", s)
        hit += n
    if hit:
        try:
            log({"event": "sanitize", "redacted": hit, "sample": str(text)[:120]})
        except Exception:
            pass
        print(
            f"[Sanitize] redacted {hit} sensitive token(s) from outgoing reply",
            flush=True,
        )
    return s


def send_tg_reply(chat_id, text):
    """发送 Telegram 群消息，失败时返回 None。"""
    text = _sanitize_reply(text)
    chat_id = str(chat_id or TG_GROUP_CHAT_ID or "").strip()
    if not text or not TG_BOT_TOKEN or not chat_id:
        print("[Telegram] send skipped: token/chat_id missing", flush=True)
        return None
    try:
        api = f"https://api.telegram.org/bot{TG_BOT_TOKEN}/sendMessage"
        body = json.dumps(
            {"chat_id": chat_id, "text": str(text), "disable_web_page_preview": True},
            ensure_ascii=False,
        ).encode("utf-8")
        req = urllib.request.Request(
            api, data=body, headers={"Content-Type": "application/json"}
        )
        with urllib.request.urlopen(req, timeout=10) as r:
            resp = json.loads(r.read(65536).decode("utf-8") or "{}")
        mid = (resp.get("result") or {}).get("message_id")
        print(f"[Telegram] Replied to {chat_id}: {text[:80]}", flush=True)
        return mid
    except Exception as e:
        print(f"[Telegram] Reply error: {e}", flush=True)
        return None


def send_tg_photo(chat_id, photo_url, caption=""):
    photo_url = str(photo_url or "").strip()
    caption = _sanitize_reply(caption)
    chat_id = str(chat_id or TG_GROUP_CHAT_ID or "").strip()
    if not photo_url or not TG_BOT_TOKEN or not chat_id:
        print("[Telegram] send photo skipped: token/chat_id/photo missing", flush=True)
        return None
    try:
        api = f"https://api.telegram.org/bot{TG_BOT_TOKEN}/sendPhoto"
        body = json.dumps(
            {
                "chat_id": chat_id,
                "photo": photo_url,
                "caption": str(caption or "")[:900],
                "disable_web_page_preview": True,
            },
            ensure_ascii=False,
        ).encode("utf-8")
        req = urllib.request.Request(
            api, data=body, headers={"Content-Type": "application/json"}
        )
        with urllib.request.urlopen(req, timeout=15) as r:
            resp = json.loads(r.read(65536).decode("utf-8") or "{}")
        mid = (resp.get("result") or {}).get("message_id")
        print(f"[Telegram] Photo sent to {chat_id}: {photo_url[:120]}", flush=True)
        return mid
    except Exception as e:
        print(f"[Telegram] Photo send error: {e}", flush=True)
        return None


def send_qq_reply(msg_type, target_id, text):
    """发送消息，返回新消息的 message_id（失败返回 None）。"""
    text = _sanitize_reply(text)
    if not text or not ONEBOT_URL:
        return None
    try:
        if msg_type == "group":
            api = f"{ONEBOT_URL}/send_group_msg"
            body = json.dumps(
                {"group_id": int(target_id), "message": str(text)}, ensure_ascii=False
            ).encode("utf-8")
        elif msg_type == "private":
            api = f"{ONEBOT_URL}/send_private_msg"
            body = json.dumps(
                {"user_id": int(target_id), "message": str(text)}, ensure_ascii=False
            ).encode("utf-8")
        else:
            return None
        req = urllib.request.Request(
            api,
            data=body,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {ONEBOT_TOKEN}",
            },
        )
        with urllib.request.urlopen(req, timeout=10) as r:
            resp = json.loads(r.read(65536).decode("utf-8") or "{}")
        mid = (resp.get("data") or {}).get("message_id")
        print(f"[OneBot] Replied to {msg_type}/{target_id}: {text[:80]}", flush=True)
        return mid
    except Exception as e:
        print(f"[OneBot] Reply error: {e}", flush=True)
        return None


def send_qq_photo(msg_type, target_id, photo_url, caption=""):
    photo_url = str(photo_url or "").strip()
    caption = _sanitize_reply(caption)
    if not photo_url or not ONEBOT_URL:
        return None
    message = []
    if caption:
        message.append({"type": "text", "data": {"text": str(caption)[:900] + "\n"}})
    message.append({"type": "image", "data": {"file": photo_url}})
    try:
        if msg_type == "group":
            api = f"{ONEBOT_URL}/send_group_msg"
            body = json.dumps(
                {"group_id": int(target_id), "message": message}, ensure_ascii=False
            ).encode("utf-8")
        elif msg_type == "private":
            api = f"{ONEBOT_URL}/send_private_msg"
            body = json.dumps(
                {"user_id": int(target_id), "message": message}, ensure_ascii=False
            ).encode("utf-8")
        else:
            return None
        req = urllib.request.Request(
            api,
            data=body,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {ONEBOT_TOKEN}",
            },
        )
        with urllib.request.urlopen(req, timeout=20) as r:
            resp = json.loads(r.read(65536).decode("utf-8") or "{}")
        mid = (resp.get("data") or {}).get("message_id")
        photo_kind = (
            "url"
            if photo_url.lower().startswith(("http://", "https://"))
            else "inline"
            if photo_url.lower().startswith("data:")
            else "other"
        )
        print(
            f"[OneBot] Photo sent to {msg_type}/{target_id}: kind={photo_kind}",
            flush=True,
        )
        return mid
    except Exception as e:
        print(f"[OneBot] Photo send error: {_redact_image_error_text(e)}", flush=True)
        return None


def _image_bool(value, default=False):
    if isinstance(value, bool):
        return value
    if value in (None, ""):
        return bool(default)
    return str(value).strip().lower() in ("1", "true", "yes", "on")


def _image_number(value, default, minimum, maximum, integer=False):
    try:
        parsed = float(value)
    except (TypeError, ValueError):
        parsed = float(default)
    parsed = min(float(maximum), max(float(minimum), parsed))
    return int(parsed) if integer else parsed


def _image_env_config():
    return {
        "enabled": True,
        "api_base_url": str(IMAGE_API_BASE_URL or "").rstrip("/"),
        "api_key": str(IMAGE_API_KEY or "").strip(),
        "model": str(IMAGE_MODEL or "gpt-image-2").strip() or "gpt-image-2",
        "size": str(IMAGE_SIZE or "1024x1024").strip() or "1024x1024",
        "timeout_seconds": _image_number(IMAGE_TIMEOUT, 180, 1, 600),
        "retry_limit": _image_number(IMAGE_RETRY_LIMIT, 2, 1, 5, integer=True),
        "retry_base_delay_seconds": _image_number(
            IMAGE_RETRY_BASE_DELAY, 3, 0, 300
        ),
        "retry_max_delay_seconds": _image_number(
            IMAGE_RETRY_MAX_DELAY, 15, 0, 300
        ),
        "cooldown_seconds": _image_number(
            IMAGE_COOLDOWN_SECONDS, 45, 0, 86400, integer=True
        ),
        "require_bind": bool(IMAGE_REQUIRE_BIND),
        "source": "environment",
    }


def _normalize_image_runtime_config(payload, fallback):
    raw = dict(payload or {}) if isinstance(payload, dict) else {}
    result = dict(fallback)
    result["enabled"] = _image_bool(raw.get("enabled"), fallback.get("enabled", True))
    base_url = str(raw.get("api_base_url") or "").strip().rstrip("/")
    if base_url.lower().startswith(("http://", "https://")):
        result["api_base_url"] = base_url
    api_key = str(raw.get("api_key") or "").strip()
    if api_key:
        result["api_key"] = api_key
    result["model"] = (
        str(raw.get("model") or fallback.get("model") or "gpt-image-2").strip()
        or "gpt-image-2"
    )
    result["size"] = (
        str(raw.get("size") or fallback.get("size") or "1024x1024").strip()
        or "1024x1024"
    )
    result["timeout_seconds"] = _image_number(
        raw.get("timeout_seconds"), fallback.get("timeout_seconds", 180), 1, 600
    )
    result["retry_limit"] = _image_number(
        raw.get("retry_limit"), fallback.get("retry_limit", 2), 1, 5, integer=True
    )
    result["retry_base_delay_seconds"] = _image_number(
        raw.get("retry_base_delay_seconds"),
        fallback.get("retry_base_delay_seconds", 3),
        0,
        300,
    )
    result["retry_max_delay_seconds"] = max(
        result["retry_base_delay_seconds"],
        _image_number(
            raw.get("retry_max_delay_seconds"),
            fallback.get("retry_max_delay_seconds", 15),
            0,
            300,
        ),
    )
    result["cooldown_seconds"] = _image_number(
        raw.get("cooldown_seconds"),
        fallback.get("cooldown_seconds", 45),
        0,
        86400,
        integer=True,
    )
    result["require_bind"] = _image_bool(
        raw.get("require_bind"), fallback.get("require_bind", False)
    )
    result["source"] = "newapi"
    return result


def _get_image_runtime_config(force=False):
    now = time.time()
    cached = _IMAGE_CONFIG_CACHE.get("value")
    if not force and cached and now < float(_IMAGE_CONFIG_CACHE.get("expires_at") or 0):
        return dict(cached)

    fallback = dict(cached or _image_env_config())
    if not IMAGE_CONFIG_FROM_NEWAPI:
        return fallback
    base = (
        os.environ.get("NEWAPI_INTERNAL_BASE_URL")
        or os.environ.get("NEWAPI_CHATOPS_BASE_URL")
        or "http://127.0.0.1:3000"
    ).rstrip("/")
    secret = _chatops_secret()
    if not base or not secret:
        return fallback

    with _IMAGE_CONFIG_LOCK:
        now = time.time()
        cached = _IMAGE_CONFIG_CACHE.get("value")
        if (
            not force
            and cached
            and now < float(_IMAGE_CONFIG_CACHE.get("expires_at") or 0)
        ):
            return dict(cached)
        fallback = dict(cached or _image_env_config())
        try:
            req = urllib.request.Request(
                _chatops_url(
                    base, "/api/agent/chatops/image-config", secret, "qq"
                ),
                headers=_chatops_headers(secret, "qq"),
            )
            with urllib.request.urlopen(
                req, timeout=IMAGE_CONFIG_FETCH_TIMEOUT_SECONDS
            ) as resp:
                body = json.loads(resp.read(65536).decode("utf-8") or "{}")
            payload = body.get("data") if isinstance(body, dict) else None
            if not isinstance(payload, dict) or body.get("success") is not True:
                raise RuntimeError("invalid image config response")
            # A successful response is always overlaid on the immutable
            # environment fallback. This lets an admin clear a database key
            # without the previous database value surviving in the cache.
            config = _normalize_image_runtime_config(payload, _image_env_config())
            _IMAGE_CONFIG_CACHE["value"] = config
            _IMAGE_CONFIG_CACHE["expires_at"] = now + IMAGE_CONFIG_CACHE_TTL_SECONDS
            log(
                {
                    "event": "image_config_refresh",
                    "status": "ok",
                    "source": "newapi",
                    "enabled": config.get("enabled"),
                    "key_configured": bool(config.get("api_key")),
                    "model": config.get("model"),
                    "size": config.get("size"),
                }
            )
            return dict(config)
        except Exception as error:
            _IMAGE_CONFIG_CACHE["value"] = fallback
            _IMAGE_CONFIG_CACHE["expires_at"] = now + min(
                5, IMAGE_CONFIG_CACHE_TTL_SECONDS
            )
            log(
                {
                    "event": "image_config_refresh",
                    "status": "fallback",
                    "source": fallback.get("source", "environment"),
                    "detail": _redact_image_error_text(error)[:200],
                }
            )
            return fallback


def _image_headers(api_key):
    return {
        "Authorization": "Bearer " + str(api_key or ""),
        "Content-Type": "application/json",
        "Accept": "application/json",
        "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
    }


_IMAGE_RETRYABLE_HTTP_STATUSES = frozenset(
    {408, 409, 425, 429, 500, 502, 503, 504, 520, 521, 522, 523, 524}
)
_IMAGE_ERROR_SAFE_FIELDS = (
    "code",
    "type",
    "message",
    "title",
    "status",
    "detail",
    "param",
)


def _redact_image_error_text(value):
    text = str(value or "")
    return _re.sub(
        r"(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b",
        "<redacted-email>",
        text,
    )


def _image_http_error_detail(error):
    try:
        raw = error.read().decode("utf-8", "ignore")[:4096]
    except Exception:
        raw = ""
    if not raw:
        return _redact_image_error_text(getattr(error, "reason", "http_error"))[:300]
    try:
        payload = json.loads(raw)
    except Exception:
        return _redact_image_error_text(raw)[:300]
    if isinstance(payload, dict) and isinstance(payload.get("error"), dict):
        payload = payload["error"]
    if isinstance(payload, dict):
        safe = {
            field: _redact_image_error_text(payload.get(field))
            for field in _IMAGE_ERROR_SAFE_FIELDS
            if payload.get(field) not in (None, "")
        }
        return json.dumps(safe, ensure_ascii=False, separators=(",", ":"))[:300]
    return _redact_image_error_text(raw)[:300]


def _image_retry_delay(attempt, config, error=None):
    delay = float(config.get("retry_base_delay_seconds") or 0) * (
        2 ** max(0, int(attempt) - 1)
    )
    headers = getattr(error, "headers", None)
    retry_after = headers.get("Retry-After") if headers else None
    if retry_after not in (None, ""):
        try:
            delay = max(delay, float(retry_after))
        except (TypeError, ValueError):
            pass
    return min(
        float(config.get("retry_max_delay_seconds") or 0), max(0.0, delay)
    )


def _call_image_service(prompt, runtime_config=None):
    config = dict(runtime_config or _get_image_runtime_config())
    if not config.get("enabled"):
        return {"ok": False, "error": "image_service_disabled"}
    api_key = str(config.get("api_key") or "").strip()
    api_base_url = str(config.get("api_base_url") or "").strip().rstrip("/")
    if not api_key or not api_base_url:
        return {"ok": False, "error": "image_service_not_configured"}
    payload = {
        "model": config.get("model") or "gpt-image-2",
        "prompt": prompt,
        "size": config.get("size") or "1024x1024",
        "n": 1,
        "response_format": "url",
    }
    last_error = "unknown"
    retry_limit = int(config.get("retry_limit") or 1)
    for attempt in range(1, retry_limit + 1):
        retry_error = None
        try:
            req = urllib.request.Request(
                api_base_url + "/images/generations",
                data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
                headers=_image_headers(api_key),
            )
            with urllib.request.urlopen(
                req, timeout=float(config.get("timeout_seconds") or 180)
            ) as resp:
                data = json.loads(resp.read().decode("utf-8") or "{}")
            items = data.get("data") or []
            first = items[0] if items else {}
            photo_url = str(first.get("url") or "").strip()
            if photo_url:
                return {
                    "ok": True,
                    "photo_url": photo_url,
                    "revised_prompt": str(first.get("revised_prompt") or "").strip(),
                }
            photo_b64 = str(first.get("b64_json") or "").strip()
            if photo_b64:
                return {
                    "ok": True,
                    "photo_url": "base64://" + photo_b64,
                    "revised_prompt": str(first.get("revised_prompt") or "").strip(),
                }
            last_error = "image_service_empty_response"
        except urllib.error.HTTPError as e:
            detail = _image_http_error_detail(e)
            last_error = f"http_{e.code}:{detail or e.reason}"
            retryable = e.code in _IMAGE_RETRYABLE_HTTP_STATUSES
            print(
                f"[Image] HTTPError attempt={attempt}/{retry_limit} status={e.code} retryable={retryable} detail={detail}",
                flush=True,
            )
            if not retryable:
                break
            retry_error = e
        except Exception as e:
            last_error = _redact_image_error_text(e)[:300]
            print(
                f"[Image] error attempt={attempt}/{retry_limit}: {last_error}",
                flush=True,
            )
        if attempt < retry_limit:
            delay = _image_retry_delay(attempt, config, retry_error)
            if delay > 0:
                time.sleep(delay)
    return {"ok": False, "error": last_error[:300]}


def _human_image_error_message(username, error, msg_type="group"):
    prefix = f"@{username} " if msg_type == "group" and username else ""
    detail = str(error or "").strip().lower()
    if not detail:
        return (
            prefix
            + "这次图没出成功，我已经记下失败了。你等会儿再发一次，我继续给你画。"
        )
    if "image_service_not_configured" in detail or "image_service_disabled" in detail:
        return (
            prefix
            + "这条生图线路现在还没配置好，我已经把错误记下来了。管理员补上图片接口地址和 key 后，我就能正常继续画。"
        )
    if "timed out" in detail or "timeout" in detail:
        return (
            prefix
            + "图我已经丢去画了，但上游这次超时了。你稍后再发一次，或者我给你切更快的生图线路。"
        )
    if "http_401" in detail or "http_403" in detail:
        return (
            prefix
            + "生图线路鉴权没过，这次不是你提示词的问题，是上游授权没配对。我已经把错误记下来了。"
        )
    if "http_429" in detail:
        return (
            prefix
            + "生图线路这会儿挤爆了，刚刚被限流了。你过一会儿再发，我继续给你画。"
        )
    if "http_5" in detail or "524" in detail:
        return (
            prefix
            + "上游生图服务刚刚掉链子了，这次是服务侧的问题，不是你不会发。你稍后再来一条，我继续画。"
        )
    if "content_policy_violation" in detail or "http_400" in detail:
        return (
            prefix
            + "这条描述没有通过图片服务检查。换一种画面描述后再发，我继续给你画。"
        )
    return (
        prefix + "这次图没出成功，我已经记下失败原因了。你稍后再发一次，我继续给你画。"
    )


def _image_delivery_fallback_message(username, photo_url, msg_type="group"):
    prefix = f"@{username} " if msg_type == "group" and username else ""
    if str(photo_url or "").lower().startswith(("http://", "https://")):
        return (
            prefix
            + f"图片已经生成，但 QQ 图片消息发送失败。你可以先打开原图：{photo_url}"
        )
    return prefix + "图片已经生成，但 QQ 图片消息发送失败。稍后再发一次，我继续处理。"


def _start_qq_image_job(
    msg_type, target_id, room_id, user_id, username, prompt, runtime_config=None
):
    def worker():
        started = time.time()
        config = dict(runtime_config or _get_image_runtime_config())
        try:
            result = _call_image_service(prompt, runtime_config=config)
            latency = int((time.time() - started) * 1000)
            log(
                {
                    "event": "image_upstream",
                    "source": "qq",
                    "room_id": room_id,
                    "user_id": user_id,
                    "status": "ok" if result.get("ok") else "error",
                    "latency_ms": latency,
                    "model": config.get("model"),
                    "size": config.get("size"),
                    "detail": str(result.get("error") or "")[:200],
                }
            )
            if result.get("ok") and result.get("photo_url"):
                photo_url = result.get("photo_url")
                caption = f"{username}，图已经生成好了：{prompt[:80]}"
                delivery_started = time.time()
                reply_mid = send_qq_photo(msg_type, target_id, photo_url, caption)
                if reply_mid is None:
                    fallback_mid = send_qq_reply(
                        msg_type,
                        target_id,
                        _image_delivery_fallback_message(username, photo_url, msg_type),
                    )
                    if msg_type == "group" and fallback_mid:
                        _remember_recent_bot_group_reply(
                            "qq",
                            room_id,
                            user_id,
                            fallback_mid,
                            reason="image_delivery_fallback",
                        )
                    log(
                        {
                            "event": "image_delivery",
                            "source": "qq",
                            "room_id": room_id,
                            "user_id": user_id,
                            "status": "delivery_error",
                            "latency_ms": int(
                                (time.time() - delivery_started) * 1000
                            ),
                            "generated_url_available": str(photo_url).lower().startswith(
                                ("http://", "https://")
                            ),
                        }
                    )
                    return
                if msg_type == "group" and reply_mid:
                    _remember_recent_bot_group_reply(
                        "qq", room_id, user_id, reply_mid, reason="image_result"
                    )
                log(
                    {
                        "event": "image_delivery",
                        "source": "qq",
                        "room_id": room_id,
                        "user_id": user_id,
                        "status": "ok",
                        "latency_ms": int((time.time() - delivery_started) * 1000),
                        "message_id": str(reply_mid),
                    }
                )
                return
            reply_mid = send_qq_reply(
                msg_type,
                target_id,
                _human_image_error_message(username, result.get("error"), msg_type),
            )
            if msg_type == "group" and reply_mid:
                _remember_recent_bot_group_reply(
                    "qq", room_id, user_id, reply_mid, reason="image_error"
                )
        except Exception as e:
            latency = int((time.time() - started) * 1000)
            detail = _redact_image_error_text(e)[:200]
            print(f"[Image] worker error: {detail}", flush=True)
            send_qq_reply(
                msg_type,
                target_id,
                _human_image_error_message(username, detail, msg_type),
            )
            log(
                {
                    "event": "image_upstream",
                    "source": "qq",
                    "room_id": room_id,
                    "user_id": user_id,
                    "status": "worker_error",
                    "latency_ms": latency,
                    "detail": detail,
                }
            )
        finally:
            _clear_image_pending("qq", room_id, user_id)

    if not _submit_background(worker, task_name="image_generate"):
        _clear_image_pending("qq", room_id, user_id)
        send_qq_reply(
            msg_type,
            target_id,
            _human_image_error_message(
                username, "系统当前任务较多，请稍后再试", msg_type
            ),
        )


def _membership_endpoint():
    base = (
        os.environ.get("NEW_API_INTERNAL_BASE_URL")
        or os.environ.get("NEW_API_CHATOPS_BASE_URL")
        or "http://127.0.0.1:3000"
    ).rstrip("/")
    path = os.environ.get(
        "NEW_API_MEMBERSHIP_EVENT_PATH", "/api/chat-membership/events"
    )
    return base + path


def _membership_secret():
    return (
        os.environ.get("NEW_API_MEMBERSHIP_EVENT_SECRET")
        or os.environ.get("CHAT_MEMBERSHIP_EVENT_SECRET")
        or os.environ.get("CHATOPS_SECRET")
        or os.environ.get("ONEBOT_TOKEN", "")
    )


_membership_retry_queue = []
_membership_retry_lock = threading.Lock()


def _queue_membership_retry(payload):
    with _membership_retry_lock:
        _membership_retry_queue.append(payload)
        max_size = int(os.environ.get("NEW_API_MEMBERSHIP_RETRY_QUEUE_MAX", "200"))
        if len(_membership_retry_queue) > max_size:
            del _membership_retry_queue[:-max_size]


def _post_membership_payload(payload):
    body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode(
        "utf-8"
    )
    headers = {"Content-Type": "application/json"}
    secret = _membership_secret().strip()
    if secret:
        digest = hmac.new(secret.encode("utf-8"), body, hashlib.sha256).hexdigest()
        headers["X-NewAPI-Signature"] = "sha256=" + digest
    req = urllib.request.Request(_membership_endpoint(), data=body, headers=headers)
    with urllib.request.urlopen(req, timeout=10) as r:
        data = r.read(65536).decode("utf-8") or "{}"
    return json.loads(data)


def flush_membership_retry_queue():
    with _membership_retry_lock:
        pending = list(_membership_retry_queue)
        _membership_retry_queue.clear()
    failed = []
    for payload in pending:
        try:
            _post_membership_payload(payload)
        except Exception as e:
            print(
                f"[MembershipRisk] retry failed event={payload.get('event_id')}: {e}",
                flush=True,
            )
            failed.append(payload)
    if failed:
        with _membership_retry_lock:
            _membership_retry_queue[:0] = failed[
                -int(os.environ.get("NEW_API_MEMBERSHIP_RETRY_QUEUE_MAX", "200")) :
            ]


def forward_membership_event(
    source,
    room_id,
    external_user_id,
    event_type,
    event_at=None,
    new_api_user_id=0,
    event_id="",
    metadata=None,
    raw_payload=None,
):
    flush_membership_retry_queue()
    event_at = int(event_at or time.time())
    payload = {
        "event_id": event_id
        or f"{source}:{room_id}:{external_user_id}:{event_type}:{event_at}",
        "source": str(source).lower().strip(),
        "room_id": str(room_id).strip(),
        "external_user_id": str(external_user_id).strip(),
        "new_api_user_id": int(new_api_user_id or 0),
        "event_type": str(event_type).lower().strip(),
        "event_at": event_at,
        "metadata": metadata or {},
        "raw_payload": raw_payload or {},
    }
    try:
        return _post_membership_payload(payload)
    except Exception:
        _queue_membership_retry(payload)
        raise


def _notify_game_director_notice(
    notice_type, gid, uid, sub_type="", operator_id="", extra=None
):
    if not _HAS_GAME_DIRECTOR:
        return None
    try:
        return _director.handle_notice(
            str(notice_type or "").strip(),
            str(gid or "").strip(),
            str(uid or "").strip(),
            sub_type=str(sub_type or "").strip(),
            operator_id=str(operator_id or "").strip(),
            extra=extra or {},
        )
    except Exception as e:
        print(
            f"[GameDirector] Notice error type={notice_type} group={gid} user={uid}: {e}",
            flush=True,
        )
        return None


def _tg_notice_event(update):
    message = update.get("message") or {}
    cm = update.get("chat_member") or update.get("my_chat_member") or {}
    chat = message.get("chat") or cm.get("chat") or {}
    chat_id = chat.get("id")
    if not chat_id:
        return None
    if message.get("new_chat_members"):
        result = None
        for user in message.get("new_chat_members") or []:
            operator_id = str((message.get("from") or {}).get("id") or "").strip()
            username = _tg_notice_display_name(user)
            ident = resolve_identity_via_newapi(
                "tg", str(chat_id), str(user.get("id")), username
            )
            result = forward_membership_event(
                source="tg",
                room_id=str(chat_id),
                external_user_id=str(user.get("id")),
                new_api_user_id=(ident or {}).get("new_api_user_id", 0) or 0,
                event_type="join",
                event_at=int(message.get("date") or time.time()),
                event_id=f"tg:{chat_id}:{user.get('id')}:new_chat_member:{message.get('message_id')}",
                metadata=_membership_notice_metadata(
                    {"sub_type": "new_chat_member", "operator_id": operator_id},
                    username,
                    ident,
                    {"tg_username": _first_nonempty((user or {}).get("username"))},
                ),
                raw_payload=update,
            )
            _notify_game_director_notice(
                "group_increase",
                chat_id,
                user.get("id"),
                sub_type="new_chat_member",
                operator_id=operator_id,
                extra=update,
            )
        return result
    if message.get("left_chat_member"):
        user = message.get("left_chat_member") or {}
        operator_id = str((message.get("from") or {}).get("id") or "").strip()
        username = _tg_notice_display_name(user)
        ident = resolve_identity_via_newapi(
            "tg", str(chat_id), str(user.get("id")), username
        )
        result = forward_membership_event(
            source="tg",
            room_id=str(chat_id),
            external_user_id=str(user.get("id")),
            new_api_user_id=(ident or {}).get("new_api_user_id", 0) or 0,
            event_type="leave",
            event_at=int(message.get("date") or time.time()),
            event_id=f"tg:{chat_id}:{user.get('id')}:left_chat_member:{message.get('message_id')}",
            metadata=_membership_notice_metadata(
                {"sub_type": "left_chat_member", "operator_id": operator_id},
                username,
                ident,
                {"tg_username": _first_nonempty((user or {}).get("username"))},
            ),
            raw_payload=update,
        )
        _notify_game_director_notice(
            "leave",
            chat_id,
            user.get("id"),
            sub_type="left_chat_member",
            operator_id=operator_id,
            extra=update,
        )
        return result
    if cm:
        newm = cm.get("new_chat_member") or {}
        user = newm.get("user") or cm.get("from") or {}
        operator_id = str((cm.get("from") or {}).get("id") or "").strip()
        status = str(newm.get("status") or "").lower()
        username = _tg_notice_display_name(user)
        ident = resolve_identity_via_newapi(
            "tg", str(chat_id), str(user.get("id")), username
        )
        if status in ("left", "kicked"):
            event_type = "kick" if status == "kicked" else "leave"
            notice_type = event_type
        elif status in ("member", "administrator", "creator"):
            event_type = "join"
            notice_type = "group_increase"
        else:
            return None
        result = forward_membership_event(
            source="tg",
            room_id=str(chat_id),
            external_user_id=str(user.get("id")),
            new_api_user_id=(ident or {}).get("new_api_user_id", 0) or 0,
            event_type=event_type,
            event_at=int(cm.get("date") or time.time()),
            event_id=f"tg:{chat_id}:{user.get('id')}:chat_member:{cm.get('date') or int(time.time())}",
            metadata=_membership_notice_metadata(
                {"sub_type": status, "operator_id": operator_id},
                username,
                ident,
                {"tg_username": _first_nonempty((user or {}).get("username"))},
            ),
            raw_payload=update,
        )
        _notify_game_director_notice(
            notice_type,
            chat_id,
            user.get("id"),
            sub_type=status,
            operator_id=operator_id,
            extra=update,
        )
        return result
    return None


def handle_onebot_notice(req, raw):
    nt = raw.get("notice_type", "")
    gid = str(raw.get("group_id", "")).strip()
    uid = str(raw.get("user_id", "")).strip()
    sub = raw.get("sub_type", "")
    print(f"[OneBot] NOTICE: type={nt} sub={sub} group={gid} user={uid}", flush=True)
    if nt == "group_increase" and gid:
        operator_id = str(raw.get("operator_id", "")).strip()
        ident, username, profile = _qq_notice_identity(gid, uid, raw)
        try:
            forward_membership_event(
                source="qq",
                room_id=gid,
                external_user_id=uid,
                new_api_user_id=(ident or {}).get("new_api_user_id", 0) or 0,
                event_type="join",
                event_at=int(raw.get("time") or time.time()),
                event_id=f"qq:{gid}:{uid}:group_increase:{raw.get('time') or int(time.time())}",
                metadata=_membership_notice_metadata(
                    {"sub_type": sub, "operator_id": operator_id},
                    username,
                    ident,
                    {
                        "qq_card": _first_nonempty((profile or {}).get("card")),
                        "qq_nickname": _first_nonempty((profile or {}).get("nickname")),
                    },
                ),
                raw_payload=raw,
            )
        except Exception as e:
            print(f"[MembershipRisk] forward join failed: {e}", flush=True)
        # Welcome + check invite
        welcome = (
            f"\U0001f44b 欢迎 [CQ:at,qq={uid}] 加入！\n\n"
            f"\U0001f680 新人三步走：\n"
            f"  1️⃣ 发送「验牌」绑定账号，领新人额度\n"
            f"  2️⃣ 发送「签到」每日签到领额度\n"
            f"  3️⃣ 发送「游戏」查看全部玩法\n\n"
            f"\U0001f3ae 热门推荐：猜拳 • 比大小 • 答题 • 夺宝奇兵\n"
            f"\U0001f4ac 直接 @我 即可 AI 对话！"
        )
        welcome_mid = send_qq_reply("group", gid, welcome)
        schedule_burn([welcome_mid], delay=30)
        if _HAS_GAME_DIRECTOR:
            try:
                _director.handle_notice(
                    "group_increase", gid, uid, sub_type=sub, operator_id=operator_id
                )
            except Exception as e:
                print(f"[GameDirector] Notice error: {e}", flush=True)
    if nt == "group_decrease" and gid:
        print(f"[OneBot] Member left: {uid}", flush=True)
        operator_id = str(raw.get("operator_id", "")).strip()
        ident, username, profile = _qq_notice_identity(gid, uid, raw)
        try:
            event_type = "kick" if str(sub).lower() == "kick" else "leave"
            forward_membership_event(
                source="qq",
                room_id=gid,
                external_user_id=uid,
                new_api_user_id=(ident or {}).get("new_api_user_id", 0) or 0,
                event_type=event_type,
                event_at=int(raw.get("time") or time.time()),
                event_id=f"qq:{gid}:{uid}:group_decrease:{raw.get('time') or int(time.time())}",
                metadata=_membership_notice_metadata(
                    {"sub_type": sub, "operator_id": operator_id},
                    username,
                    ident,
                    {
                        "qq_card": _first_nonempty((profile or {}).get("card")),
                        "qq_nickname": _first_nonempty((profile or {}).get("nickname")),
                    },
                ),
                raw_payload=raw,
            )
        except Exception as e:
            print(f"[MembershipRisk] forward leave failed: {e}", flush=True)
        _notify_game_director_notice(
            "group_decrease", gid, uid, sub_type=sub, operator_id=operator_id, extra=raw
        )
    return send_json(req, 200, {"ok": True, "notice_handled": True})


def handle_onebot(req):
    raw = read_json(req)
    post_type = str(raw.get("post_type") or "").strip().lower()
    if post_type == "meta_event":
        return send_json(req, 200, {"ok": True, "ignored": "meta_event"})
    if post_type == "notice":
        return handle_onebot_notice(req, raw)
    if post_type and post_type != "message":
        print(
            f"[OneBot] ignored post_type={post_type} keys={sorted(raw.keys())[:12]}",
            flush=True,
        )
        return send_json(req, 200, {"ok": True, "ignored": f"post_type_{post_type}"})
    mt = str(raw.get("message_type", "") or "").strip().lower()
    gid = str(raw.get("group_id", "")).strip()
    if not mt:
        mt = (
            "group"
            if gid
            else ("private" if str(raw.get("user_id", "")).strip() else "")
        )
    conversation_room_id = gid if mt == "group" else ""
    uid = str(raw.get("user_id", "")).strip()
    msg = _coerce_onebot_message_text(
        raw.get("message", ""), raw.get("raw_message", "")
    )
    image_prompt = detect_image_prompt(msg)
    mid = str(raw.get("message_id", ""))
    stable_event_key = _source_event_key("qq", conversation_room_id or uid, mid)
    game_trigger = _director.classify_text(msg) if _HAS_GAME_DIRECTOR else None
    print(
        f"[OneBot] EVENT: type={mt} group={gid} user={uid} msg={msg[:60]}", flush=True
    )
    # 去重：同一 message_id 短期内重复推送直接忽略，避免重复回复。
    if _is_duplicate_message(mid):
        return send_json(req, 200, {"ok": True, "ignored": "duplicate_message"})
    username = raw.get("sender", {}).get("card") or raw.get("sender", {}).get(
        "nickname", uid
    )
    observed_role = raw.get("sender", {}).get("role", "member")
    trusted_group_admin, verified_role = (
        _trusted_qq_admin(conversation_room_id, uid) if mt == "group" else (False, "")
    )
    role = verified_role or observed_role
    configured_admin = _is_source_admin("qq", uid, username)
    is_admin = bool(configured_admin or trusted_group_admin)
    admin_source = (
        "configured"
        if configured_admin
        else ("onebot_member_lookup" if trusted_group_admin else "none")
    )
    target_meta = _qq_target_meta(raw, msg)
    raw_for_capture = dict(raw)
    raw_for_capture.update(target_meta)
    _capture_chatops_message(
        "qq",
        conversation_room_id,
        uid,
        username,
        msg,
        message_id=mid,
        user_role=role,
        is_admin=is_admin,
        raw=raw_for_capture,
    )
    effective_room_id, room_context_reason = _resolve_effective_room_context(
        "qq", mt, conversation_room_id, uid, username, msg, is_admin=is_admin
    )
    # 群消息风控：对所有群消息（不只@bot的）做敏感内容扫描，命中违规则撤回+提醒（群主/管理员豁免）。
    if mt == "group":
        try:
            if handle_group_risk(raw, gid, uid, msg, mid):
                return send_json(req, 200, {"ok": True, "risk_handled": True})
        except Exception as e:
            print(f"[Risk] error: {e}", flush=True)
    code = _is_bind_command(msg)
    if code:
        bind = confirm_bind_via_newapi("qq", conversation_room_id, uid, username, code)
        if bind.get("ok"):
            reply = _render_bind_success_reply("qq", conversation_room_id, username)
        else:
            reply = f"@{username} ❌ 绑定失败：{bind.get('message') or '绑定码无效、已过期或已使用'}"
        reply_mid = send_qq_reply(
            mt, conversation_room_id if mt == "group" else uid, reply
        )
        if mt == "group" and reply_mid:
            _remember_recent_bot_group_reply(
                "qq", conversation_room_id, uid, reply_mid, reason="bind"
            )
        # 阅后即焚：撤回 bot 回复 + 用户的绑定命令（含绑定码，敏感）。
        burn = [reply_mid]
        if mt == "group" and mid:
            burn.append(mid)
        _schedule_sensitive_burn(burn)
        return send_json(req, 200, {"ok": True, "bind_handled": True, "bind": bind})
    if detect_image_status_query(msg):
        room_id = gid if mt == "group" else uid
        pending = _get_image_pending("qq", room_id, uid)
        if pending:
            wait_seconds = max(
                1, int(time.time() - float(pending.get("started_at") or time.time()))
            )
            pending_prompt = pending.get("prompt") or "未命名任务"
            reply = (
                f"@{username} 还在画，已经跑了 {wait_seconds} 秒，内容是：{pending_prompt}。画好我会直接把图发给你。"
                if mt == "group"
                else f"还在画，已经跑了 {wait_seconds} 秒，内容是：{pending_prompt}。画好我会直接把图发给你。"
            )
            reply_mid = send_qq_reply(mt, gid if mt == "group" else uid, reply)
            if mt == "group" and reply_mid:
                _remember_recent_bot_group_reply(
                    "qq", conversation_room_id, uid, reply_mid, reason="image_status"
                )
            return send_json(
                req,
                200,
                {"ok": True, "image_status": "pending", "wait_seconds": wait_seconds},
            )
    # 群聊降噪：群消息只有 @bot 时才进入 AI 对话；命令（签到/验牌/绑定）在上面已处理，不受影响。
    # 私聊不受限制。这样避免 bot 对群里每条普通消息都回复造成刷屏。
    if mt == "group":
        self_id = str(raw.get("self_id", "") or "")
        raw_message = str(raw.get("raw_message", "") or raw.get("message", "") or "")
        at_me = False
        if self_id and (
            f"[CQ:at,qq={self_id}]" in raw_message
            or f"CQ:at,qq={self_id}" in raw_message
        ):
            at_me = True
        # 兼容部分上报把 at 放进 message 数组已被转成文本的情况
        if not at_me and self_id and ("qq=" + self_id) in raw_message:
            at_me = True
        if not at_me:
            # 游戏触发词不需要@bot（签到/验牌/骰子等直接发就能玩）。
            # 管理员的自然语言运维/群管指令也不强制 @bot，避免“有权限但收不到指令”。
            game_followup = False
            if _HAS_GAME_DIRECTOR:
                try:
                    game_followup = bool(
                        _director.is_game_followup(
                            msg,
                            BOT_PLATFORM or "qq",
                            effective_room_id or conversation_room_id,
                            uid,
                        )
                    )
                except Exception:
                    game_followup = False
            if game_trigger or game_followup:
                pass  # game trigger, let it through
            elif image_prompt and IMAGE_GROUP_NO_AT:
                pass
            elif is_admin and _is_admin_direct_intent(msg):
                pass
            else:
                return send_json(req, 200, {"ok": True, "ignored": "group_not_at_bot"})
    force_ad_lookup = bool(is_admin and mt == "private" and _is_ad_lookup_followup(msg))
    ad_lookup_target_room = (
        effective_room_id if mt == "private" else conversation_room_id
    )
    if (
        (force_ad_lookup or _is_ad_lookup_query(msg))
        and mt == "private"
        and is_admin
        and not ad_lookup_target_room
    ):
        reply = _render_missing_room_reply("qq")
        send_qq_reply(mt, uid, reply)
        _append_chat_history("qq", conversation_room_id, uid, msg, reply)
        log(
            {
                "event": "ad_lookup_missing_room",
                "source": "qq",
                "user_id": uid,
                "is_admin": is_admin,
                "message_type": mt,
            }
        )
        return send_json(
            req,
            200,
            {"ok": True, "ad_lookup_handled": False, "reason": "missing_room_context"},
        )
    ad_lookup = _handle_recent_ad_lookup(
        "qq",
        ad_lookup_target_room,
        uid,
        username,
        msg,
        message_id=mid,
        is_admin=is_admin,
        force=force_ad_lookup,
    )
    if ad_lookup:
        if ad_lookup_target_room:
            _remember_ad_lookup_context("qq", uid, ad_lookup_target_room)
        target = conversation_room_id if mt == "group" else uid
        reply_text = ad_lookup["reply"]
        if mt != "group" and ad_lookup_target_room:
            reply_text = f"我先按 QQ 群 {ad_lookup_target_room} 查。\n{reply_text}"
        elif mt == "group":
            reply_text = f"@{username} {reply_text}"
        reply_mid = send_qq_reply(mt, target, reply_text)
        if mt == "group" and reply_mid:
            _remember_recent_bot_group_reply(
                "qq", conversation_room_id, uid, reply_mid, reason="ad_lookup"
            )
        if mt == "group" and reply_mid and _burn_after_seconds() > 0:
            schedule_burn([reply_mid])
        _append_chat_history("qq", conversation_room_id, uid, msg, reply_text)
        log(
            {
                "event": "ad_lookup",
                "source": "qq",
                "room_id": ad_lookup_target_room,
                "conversation_room_id": conversation_room_id,
                "room_context": room_context_reason,
                "user_id": uid,
                "is_admin": is_admin,
                "matches": (ad_lookup.get("ad_lookup") or {}).get("match_count", 0),
            }
        )
        return send_json(
            req,
            200,
            {
                "ok": True,
                "ad_lookup_handled": True,
                "sent_text": ad_lookup["reply"][:200],
                "ad_lookup": ad_lookup.get("ad_lookup"),
            },
        )
    identity_room_id = effective_room_id or conversation_room_id
    ident = resolve_identity_via_newapi("qq", identity_room_id, uid, username)
    if image_prompt:
        room_id = gid if mt == "group" else uid
        image_config = _get_image_runtime_config()
        if image_config.get("require_bind") and not bool(
            ident.get("user_bound") or ident.get("bound")
        ):
            reply = _render_bind_required_reply(
                "qq", identity_room_id or room_id, username, mt
            )
            reply_mid = send_qq_reply(mt, gid if mt == "group" else uid, reply)
            if mt == "group" and reply_mid:
                _remember_recent_bot_group_reply(
                    "qq",
                    conversation_room_id,
                    uid,
                    reply_mid,
                    reason="image_bind_required",
                )
            return send_json(
                req,
                200,
                {"ok": True, "image_handled": False, "reason": "bind_required"},
            )
        if (
            not image_config.get("enabled")
            or not image_config.get("api_key")
            or not image_config.get("api_base_url")
        ):
            reason = (
                "image_service_disabled"
                if not image_config.get("enabled")
                else "image_service_not_configured"
            )
            reply_mid = send_qq_reply(
                mt,
                gid if mt == "group" else uid,
                _human_image_error_message(
                    username, reason, mt
                ),
            )
            if mt == "group" and reply_mid:
                _remember_recent_bot_group_reply(
                    "qq",
                    conversation_room_id,
                    uid,
                    reply_mid,
                    reason="image_not_configured",
                )
            return send_json(
                req,
                200,
                {
                    "ok": True,
                    "image_handled": False,
                    "reason": reason,
                },
            )
        cooldown_left = _image_cooldown_left(
            "qq", room_id, uid, image_config.get("cooldown_seconds")
        )
        if cooldown_left > 0:
            reply = (
                f"@{username} 你画得太快啦，等 {cooldown_left} 秒后再发一次。"
                if mt == "group"
                else f"你画得太快啦，等 {cooldown_left} 秒后再发一次。"
            )
            reply_mid = send_qq_reply(mt, gid if mt == "group" else uid, reply)
            if mt == "group" and reply_mid:
                _remember_recent_bot_group_reply(
                    "qq", conversation_room_id, uid, reply_mid, reason="image_cooldown"
                )
            return send_json(
                req,
                200,
                {
                    "ok": True,
                    "image_handled": False,
                    "reason": "cooldown",
                    "cooldown_left": cooldown_left,
                },
            )
        _set_image_cooldown(
            "qq", room_id, uid, image_config.get("cooldown_seconds")
        )
        _set_image_pending("qq", room_id, uid, image_prompt)
        ack = (
            f"@{username} 收到，这就开始画：{image_prompt[:60]}"
            if mt == "group"
            else f"收到，这就开始画：{image_prompt[:60]}"
        )
        reply_mid = send_qq_reply(mt, gid if mt == "group" else uid, ack)
        if mt == "group" and reply_mid:
            _remember_recent_bot_group_reply(
                "qq", conversation_room_id, uid, reply_mid, reason="image_ack"
            )
        _start_qq_image_job(
            mt,
            gid if mt == "group" else uid,
            room_id,
            uid,
            username,
            image_prompt,
            runtime_config=image_config,
        )
        return send_json(
            req, 200, {"ok": True, "image_handled": True, "prompt": image_prompt[:120]}
        )
    ctx = {
        "site_id": SITE_ID,
        "site_name": SITE_NAME,
        "platform": BOT_PLATFORM or "qq",
        "group_id": effective_room_id or conversation_room_id,
        "effective_room_id": effective_room_id or "",
        "room_context_reason": room_context_reason or "",
        "user_id": uid,
        "username": username,
        "new_api_user_id": ident.get("new_api_user_id", 0) or 0,
        "new_api_token": "",
        "text": msg,
        "quota_balance": ident.get("quota_balance", 0) or 0,
        "is_admin": is_admin,
        "user_bound": bool(ident.get("user_bound") or ident.get("bound")),
    }
    if _is_agent_permission_query(msg):
        reply = _agent_permission_reply(is_admin)
        target = gid if mt == "group" else uid
        reply_mid = send_qq_reply(
            mt, target, f"@{username} {reply}" if mt == "group" else reply
        )
        if mt == "group" and reply_mid:
            _remember_recent_bot_group_reply(
                "qq", conversation_room_id, uid, reply_mid, reason="permission_query"
            )
        if mt == "group" and reply_mid and _burn_after_seconds() > 0:
            schedule_burn([reply_mid])
        return send_json(
            req,
            200,
            {"ok": True, "permission_query_handled": True, "sent_text": reply[:200]},
        )
    if _HAS_GAME_DIRECTOR:
        try:
            anns = []
            result = _director.handle(ctx)
            if result and result.get("reply"):
                pipeline = run_game_action_pipeline(
                    result.get("actions") or [],
                    "qq",
                    effective_room_id or conversation_room_id,
                    uid,
                    event_key=stable_event_key,
                )
                action_results = pipeline.get("results") or []
                result = _finalize_game_reply(result, action_results)
                target = conversation_room_id if mt == "group" else uid
                reply_mid = send_qq_reply(mt, target, result["reply"])
                if mt == "group" and reply_mid:
                    _remember_recent_bot_group_reply(
                        "qq", conversation_room_id, uid, reply_mid, reason="game_reply"
                    )
                if mt == "group" and reply_mid and _burn_after_seconds() > 0:
                    schedule_burn([reply_mid])
                print(
                    f"[GameDirector] Handled: {msg[:50]} actions={len(action_results)}",
                    flush=True,
                )
                return send_json(
                    req,
                    200,
                    {
                        "ok": True,
                        "game_handled": True,
                        "sent_text": result["reply"][:200],
                        "action_results": action_results,
                    },
                )
        except Exception as e:
            print(f"[GameDirector] Error: {e}", flush=True)
    if game_trigger:
        print(
            f"[GameDirector] Trigger without result: {msg[:50]} trigger={game_trigger}",
            flush=True,
        )
        fallback = _render_game_unavailable_reply(
            "qq",
            effective_room_id or conversation_room_id,
            username,
            msg,
            is_admin=is_admin,
            user_bound=bool(ident.get("user_bound") or ident.get("bound")),
        )
        target = conversation_room_id if mt == "group" else uid
        reply_mid = send_qq_reply(mt, target, fallback)
        if mt == "group" and reply_mid:
            _remember_recent_bot_group_reply(
                "qq", conversation_room_id, uid, reply_mid, reason="game_unavailable"
            )
        if mt == "group" and reply_mid and _burn_after_seconds() > 0:
            schedule_burn([reply_mid])
        return send_json(
            req,
            200,
            {
                "ok": True,
                "game_handled": False,
                "ignored": "game_trigger_no_result",
                "trigger": game_trigger,
                "sent_text": fallback[:200],
            },
        )
    try:
        payload = {
            "message": msg,
            "user_id": uid,
            "platform": BOT_PLATFORM or "qq",
            "group_id": effective_room_id or conversation_room_id,
            "effective_room_id": effective_room_id or "",
            "conversation_group_id": conversation_room_id,
            "room_context_reason": room_context_reason or "",
            "message_type": mt,
            "source": "qq",
            "username": username,
            "issuer_role": role,
            "is_admin": is_admin,
            "_trusted_is_admin": is_admin,
            "_trusted_user_id": uid,
            "_stable_event_key": stable_event_key,
            "admin_source": admin_source,
            "is_owner": role == "owner",
            "message_id": mid,
            **target_meta,
        }
        history = _get_chat_history("qq", conversation_room_id, uid, 12)
        result = call_model(payload, history)
        result = _guard_llm_result(payload, result)
        if result:
            actions = result.get("actions") or []
            action_results = []
            if actions:
                action_results = run_game_actions_via_newapi(
                    actions,
                    "qq",
                    effective_room_id or conversation_room_id,
                    uid,
                    username=username,
                    user_role=role,
                    is_admin=is_admin,
                    message_id=mid,
                    raw=raw_for_capture,
                )
                summaries = _action_user_summaries(actions, action_results)
                if summaries:
                    result["reply"] = (result.get("reply") or "").strip()
                    result["reply"] = (
                        result["reply"] + "\n" if result["reply"] else ""
                    ) + "\n".join(summaries)
            if not result.get("reply") and action_results:
                result["reply"] = "搞定，已经处理好了。"
        if result and result.get("reply"):
            target = conversation_room_id if mt == "group" else uid
            reply_mid = send_qq_reply(mt, target, result["reply"])
            if mt == "group" and reply_mid:
                _remember_recent_bot_group_reply(
                    "qq", conversation_room_id, uid, reply_mid, reason="ai_reply"
                )
            if mt == "group" and reply_mid and _burn_after_seconds() > 0:
                schedule_burn([reply_mid])
            _append_chat_history("qq", conversation_room_id, uid, msg, result["reply"])
            log(
                {
                    "event": "plan",
                    "path": "/onebot",
                    "source": "qq",
                    "role": payload.get("issuer_role"),
                    "risk": result.get("risk"),
                    "approval": result.get("requires_approval"),
                    "room_id": effective_room_id or conversation_room_id,
                    "conversation_room_id": conversation_room_id,
                    "room_context": room_context_reason,
                    "user_id": uid,
                    "message_id": mid,
                    "admin_source": admin_source,
                }
            )
            return send_json(
                req,
                200,
                {
                    "ok": True,
                    "ai_handled": True,
                    "sent_text": result["reply"][:200],
                    "action_results": action_results
                    if "action_results" in locals()
                    else [],
                },
            )
    except Exception as e:
        print(f"[AI] Error: {e}", flush=True)
        traceback.print_exc()
        fallback = _human_ai_error_reply(username, msg, mt)
        target = conversation_room_id if mt == "group" else uid
        if fallback and target:
            reply_mid = send_qq_reply(mt, target, fallback)
            if mt == "group" and reply_mid:
                _remember_recent_bot_group_reply(
                    "qq",
                    conversation_room_id,
                    uid,
                    reply_mid,
                    reason="ai_error_fallback",
                )
            if mt == "group" and reply_mid and _burn_after_seconds() > 0:
                schedule_burn([reply_mid])
            _append_chat_history("qq", conversation_room_id, uid, msg, fallback)
            log(
                {
                    "event": "ai_error_fallback",
                    "source": "qq",
                    "room_id": effective_room_id or conversation_room_id,
                    "conversation_room_id": conversation_room_id,
                    "room_context": room_context_reason,
                    "user_id": uid,
                    "is_admin": is_admin,
                    "error": str(e)[:200],
                }
            )
            return send_json(
                req,
                200,
                {
                    "ok": True,
                    "ai_handled": False,
                    "fallback": "model_error",
                    "sent_text": fallback[:200],
                },
            )
    if mt == "private" and uid and str(msg or "").strip():
        fallback = "我收到了，不过你这句还差半步信息。你直接补一句要查哪个群、哪个成员、要做什么动作，我就顺着继续给你办。"
        send_qq_reply("private", uid, fallback)
        _append_chat_history("qq", conversation_room_id, uid, msg, fallback)
        log(
            {
                "event": "private_no_handler_fallback",
                "source": "qq",
                "user_id": uid,
                "room_id": effective_room_id or conversation_room_id,
                "room_context": room_context_reason,
            }
        )
        return send_json(
            req, 200, {"ok": True, "no_handler": True, "private_fallback": True}
        )
    return send_json(req, 200, {"ok": True, "no_handler": True})


def handle_with_games(payload):
    if not _HAS_GAME_DIRECTOR:
        return None
    try:
        gid = str(payload.get("group_id", payload.get("chat_id", "")))
        platform = str(
            payload.get("platform")
            or payload.get("source")
            or BOT_PLATFORM
            or "unknown"
        ).lower()
        source = str(payload.get("source") or platform or "unknown").lower()
        if source == "telegram":
            source = "tg"
        if platform == "telegram":
            platform = "tg"
        uid = str(
            payload.get(
                "user_id", payload.get("issuer_id", payload.get("user_external_id", ""))
            )
        )
        username = str(payload.get("username", payload.get("issuer_name", uid)))
        stable_event_key = str(
            payload.get("_stable_event_key") or ""
        ).strip() or _source_event_key(
            source,
            gid or uid,
            payload.get("message_id") or payload.get("update_id") or "",
        )
        text_for_bind = str(
            payload.get("text")
            or payload.get("message")
            or payload.get("command")
            or payload.get("raw_command")
            or ""
        )
        game_trigger = _director.classify_text(text_for_bind)
        code = _is_bind_command(text_for_bind)
        if code:
            bind = confirm_bind_via_newapi(source, gid, uid, username, code)
            if bind.get("ok"):
                return {
                    "reply": _render_bind_success_reply(source, gid, username),
                    "risk": "low",
                    "requires_approval": False,
                    "actions": [],
                    "notes": "bind_confirmed",
                    "game_handled": True,
                }
            return {
                "reply": f"@{username} ❌ 绑定失败：{bind.get('message') or '绑定码无效、已过期或已使用'}",
                "risk": "low",
                "requires_approval": False,
                "actions": [],
                "notes": "bind_failed",
                "game_handled": True,
            }
        ident = {}
        if not (
            payload.get("new_api_user_id", 0)
            or payload.get("user_bound", False)
            or payload.get("community_bound", False)
        ):
            ident = resolve_identity_via_newapi(source, gid, uid, username)
        ctx = {
            "site_id": SITE_ID,
            "site_name": SITE_NAME,
            "platform": platform,
            "group_id": gid,
            "user_id": uid,
            "username": username,
            "new_api_user_id": payload.get("new_api_user_id", 0)
            or ident.get("new_api_user_id", 0)
            or 0,
            "new_api_token": payload.get("new_api_token", "") or "",
            "text": str(
                payload.get("text")
                or payload.get("message")
                or payload.get("command")
                or payload.get("raw_command")
                or ""
            ),
            "quota_balance": payload.get("quota_balance", 0)
            or ident.get("quota_balance", 0)
            or 0,
            "is_admin": bool(payload.get("_trusted_is_admin", False)),
            "_trusted_is_admin": bool(payload.get("_trusted_is_admin", False)),
            "admin_source": str(payload.get("admin_source") or "none"),
            "user_bound": bool(
                payload.get("user_bound", payload.get("community_bound", False))
                or ident.get("user_bound")
                or ident.get("bound")
            ),
        }
        result = _director.handle(ctx)
        if result and result.get("reply"):
            pipeline = run_game_action_pipeline(
                result.get("actions") or [],
                source,
                gid,
                uid,
                event_key=stable_event_key,
            )
            action_results = pipeline.get("results") or []
            result = _finalize_game_reply(result, action_results)
            failed_actions = [
                r
                for r in (action_results or [])
                if isinstance(r, dict)
                and not bool(r.get("success", r.get("ok", False)))
            ]
            print(
                f"[GameDirector] Handled: {str(payload.get('text') or payload.get('command') or payload.get('message') or '')[:50]} actions={len(action_results)} failed_actions={len(failed_actions)}",
                flush=True,
            )
            if failed_actions:
                print(
                    f"[GameDirector] action failures detail={json.dumps(failed_actions, ensure_ascii=False)[:1000]}",
                    flush=True,
                )
            return result
        if game_trigger:
            return {
                "reply": _render_game_unavailable_reply(
                    source,
                    gid,
                    username,
                    text_for_bind,
                    is_admin=bool(payload.get("is_admin", False)),
                    user_bound=bool(
                        payload.get("user_bound", False)
                        or payload.get("community_bound", False)
                        or ident.get("user_bound")
                        or ident.get("bound")
                    ),
                ),
                "risk": "low",
                "requires_approval": False,
                "actions": [],
                "notes": "game_trigger_no_result",
                "game_handled": True,
            }
        return None
    except Exception as e:
        print(f"[GameDirector] Error: {e}", flush=True)
        return None


def handle_tg_message(update):
    msg = update.get("message") or update.get("edited_message") or {}
    if not msg:
        return None
    chat = msg.get("chat") or {}
    from_user = msg.get("from") or {}
    chat_id = str(chat.get("id") or TG_GROUP_CHAT_ID or "").strip()
    user_id = str(from_user.get("id") or "").strip()
    text = str(msg.get("text") or msg.get("caption") or "").strip()
    if not chat_id or not user_id or not text:
        return {"ok": True, "ignored": "empty_tg_message"}
    message_id = str(msg.get("message_id") or "")
    stable_event_key = _telegram_event_key(update)
    username = _tg_display_name(from_user)
    chat_type = str(chat.get("type") or "").lower()
    conversation_room_id = chat_id if chat_type in ("group", "supergroup") else ""
    role = "private"
    tg_group_admin = False
    if conversation_room_id:
        role, tg_group_admin = _tg_get_member_role(conversation_room_id, user_id)
    is_admin = _is_source_admin("tg", user_id, username, role) or tg_group_admin
    admin_source = (
        "configured"
        if _is_source_admin("tg", user_id, username)
        else ("telegram_get_chat_member" if tg_group_admin else "none")
    )
    target_meta = _tg_target_meta(msg)
    raw_for_capture = dict(update)
    raw_for_capture.update(target_meta)
    _capture_chatops_message(
        "tg",
        conversation_room_id,
        user_id,
        username,
        text,
        message_id=message_id,
        user_role=role or "member",
        is_admin=is_admin,
        raw=raw_for_capture,
    )
    effective_room_id, room_context_reason = _resolve_effective_room_context(
        "tg",
        chat_type or "private",
        conversation_room_id,
        user_id,
        username,
        text,
        is_admin=is_admin,
    )

    bot_username = os.environ.get("TG_BOT_USERNAME", "").strip().lstrip("@")
    mentioned = bool(bot_username and (("@" + bot_username.lower()) in text.lower()))
    admin_moderation = is_admin and _is_admin_direct_intent(text)
    if chat_type in ("group", "supergroup") and not (
        mentioned or admin_moderation or text.startswith("/")
    ):
        game_trigger = _director.classify_text(text) if _HAS_GAME_DIRECTOR else None
        if not game_trigger:
            return {"ok": True, "ignored": "tg_group_not_for_bot"}

    force_ad_lookup = bool(
        is_admin and chat_type == "private" and _is_ad_lookup_followup(text)
    )
    ad_lookup_target_room = (
        effective_room_id if chat_type == "private" else conversation_room_id
    )
    if (
        (force_ad_lookup or _is_ad_lookup_query(text))
        and chat_type == "private"
        and is_admin
        and not ad_lookup_target_room
    ):
        reply = _render_missing_room_reply("tg")
        send_tg_reply(chat_id, reply)
        _append_chat_history("tg", conversation_room_id, user_id, text, reply)
        log(
            {
                "event": "ad_lookup_missing_room",
                "source": "tg",
                "user_id": user_id,
                "is_admin": is_admin,
                "message_type": chat_type,
            }
        )
        return {
            "ok": True,
            "tg_ad_lookup_handled": False,
            "reason": "missing_room_context",
        }
    ad_lookup = _handle_recent_ad_lookup(
        "tg",
        ad_lookup_target_room,
        user_id,
        username,
        text,
        message_id=message_id,
        is_admin=is_admin,
        force=force_ad_lookup,
    )
    if ad_lookup:
        if ad_lookup_target_room:
            _remember_ad_lookup_context("tg", user_id, ad_lookup_target_room)
        reply_text = ad_lookup["reply"]
        if chat_type == "private" and ad_lookup_target_room:
            reply_text = f"我先按 TG 群 {ad_lookup_target_room} 查。\n{reply_text}"
        send_tg_reply(chat_id, reply_text)
        _append_chat_history("tg", conversation_room_id, user_id, text, reply_text)
        log(
            {
                "event": "ad_lookup",
                "source": "tg",
                "room_id": ad_lookup_target_room,
                "conversation_room_id": conversation_room_id,
                "room_context": room_context_reason,
                "user_id": user_id,
                "is_admin": is_admin,
                "matches": (ad_lookup.get("ad_lookup") or {}).get("match_count", 0),
            }
        )
        return {
            "ok": True,
            "tg_ad_lookup_handled": True,
            "sent_text": ad_lookup["reply"][:200],
            "ad_lookup": ad_lookup.get("ad_lookup"),
        }

    ident = resolve_identity_via_newapi(
        "tg", effective_room_id or conversation_room_id, user_id, username
    )
    ctx = {
        "site_id": SITE_ID,
        "site_name": SITE_NAME,
        "platform": "tg",
        "group_id": effective_room_id or conversation_room_id,
        "effective_room_id": effective_room_id or "",
        "room_context_reason": room_context_reason or "",
        "user_id": user_id,
        "username": username,
        "new_api_user_id": ident.get("new_api_user_id", 0) or 0,
        "new_api_token": "",
        "text": text,
        "quota_balance": ident.get("quota_balance", 0) or 0,
        "is_admin": is_admin,
        "_trusted_is_admin": is_admin,
        "admin_source": admin_source,
        "user_bound": bool(ident.get("user_bound") or ident.get("bound")),
    }
    if _HAS_GAME_DIRECTOR:
        try:
            result = _director.handle(ctx)
            if result and result.get("reply"):
                pipeline = run_game_action_pipeline(
                    result.get("actions") or [],
                    "tg",
                    effective_room_id or conversation_room_id,
                    user_id,
                    event_key=stable_event_key,
                )
                action_results = pipeline.get("results") or []
                result = _finalize_game_reply(result, action_results)
                send_tg_reply(chat_id, result["reply"])
                return {
                    "ok": True,
                    "tg_game_handled": True,
                    "sent_text": result["reply"][:200],
                    "action_results": action_results,
                }
        except Exception as e:
            print(f"[Telegram][GameDirector] Error: {e}", flush=True)

    try:
        payload = {
            "message": text,
            "text": text,
            "user_id": user_id,
            "platform": "tg",
            "source": "tg",
            "group_id": effective_room_id or conversation_room_id,
            "chat_id": effective_room_id or conversation_room_id,
            "effective_room_id": effective_room_id or "",
            "conversation_chat_id": chat_id,
            "conversation_group_id": conversation_room_id,
            "room_context_reason": room_context_reason or "",
            "message_type": chat_type or "private",
            "username": username,
            "issuer_role": role or "member",
            "is_admin": is_admin,
            "_trusted_is_admin": is_admin,
            "_trusted_user_id": user_id,
            "_stable_event_key": stable_event_key,
            "admin_source": admin_source,
            "message_id": message_id,
            **target_meta,
        }
        if _is_agent_permission_query(text):
            reply = _agent_permission_reply(is_admin)
            send_tg_reply(chat_id, reply)
            return {
                "ok": True,
                "permission_query_handled": True,
                "sent_text": reply[:200],
            }
        history = _get_chat_history("tg", conversation_room_id, user_id, 12)
        result = call_model(payload, history)
        result = _guard_llm_result(payload, result)
        action_results = []
        if result:
            actions = result.get("actions") or []
            if actions:
                action_results = run_game_actions_via_newapi(
                    actions,
                    "tg",
                    effective_room_id or conversation_room_id,
                    user_id,
                    username=username,
                    user_role=role or "member",
                    is_admin=is_admin,
                    message_id=message_id,
                    raw=raw_for_capture,
                )
                summaries = _action_user_summaries(actions, action_results)
                if summaries:
                    result["reply"] = (result.get("reply") or "").strip()
                    result["reply"] = (
                        result["reply"] + "\n" if result["reply"] else ""
                    ) + "\n".join(summaries)
            if not result.get("reply") and action_results:
                result["reply"] = "搞定，已经处理好了。"
        if result and result.get("reply"):
            send_tg_reply(chat_id, result["reply"])
            _append_chat_history(
                "tg", conversation_room_id, user_id, text, result["reply"]
            )
            log(
                {
                    "event": "plan",
                    "path": "/telegram",
                    "source": "tg",
                    "role": role,
                    "risk": result.get("risk"),
                    "approval": result.get("requires_approval"),
                    "room_id": effective_room_id or conversation_room_id,
                    "conversation_room_id": conversation_room_id,
                    "room_context": room_context_reason,
                }
            )
            return {
                "ok": True,
                "tg_ai_handled": True,
                "sent_text": result["reply"][:200],
                "action_results": action_results,
            }
    except Exception as e:
        print(f"[Telegram][AI] Error: {e}", flush=True)
        return {"ok": False, "error": str(e)}
    if chat_type == "private" and user_id and str(text or "").strip():
        fallback = "我收到了，不过你这句还差半步信息。你直接补一句要查哪个群、哪个成员、要做什么动作，我就顺着继续给你办。"
        send_tg_reply(chat_id, fallback)
        _append_chat_history("tg", conversation_room_id, user_id, text, fallback)
        log(
            {
                "event": "private_no_handler_fallback",
                "source": "tg",
                "user_id": user_id,
                "room_id": effective_room_id or conversation_room_id,
                "room_context": room_context_reason,
            }
        )
        return {"ok": True, "ignored": "tg_no_handler", "private_fallback": True}
    return {"ok": True, "ignored": "tg_no_handler"}


def handle_admin(path, req):
    if not _HAS_ADMIN_PANEL:
        return 500, {"ok": False, "error": "admin_panel_not_loaded"}
    try:
        n = int(req.headers.get("Content-Length", "0") or "0")
        raw = req.rfile.read(min(n, 500_000))
        payload = json.loads(raw.decode("utf-8") or "{}")
    except Exception:
        payload = {}
    if req.command == "GET":
        query = parse_qs(urlparse(req.path).query, keep_blank_values=True)
        for key, values in query.items():
            if key not in payload and values:
                payload[key] = values[0] if len(values) == 1 else values
    return GameAdminHandler.handle(path, payload, req.command)


class Handler(BaseHTTPRequestHandler):
    server_version = "newapi-hermes-adapter/0.3"

    def log_message(self, fmt, *args):
        return

    def do_OPTIONS(self):
        if self.path.startswith("/game-admin") or self.path.startswith("/admin"):
            self.send_response(204)
            self.send_header("Access-Control-Allow-Origin", "*")
            self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            self.send_header(
                "Access-Control-Allow-Headers",
                "Content-Type, X-Hermes-Key, X-GameAdmin-Key, Authorization",
            )
            self.send_header("Access-Control-Max-Age", "86400")
            self.end_headers()
        else:
            self.send_response(405)
            self.end_headers()

    def do_GET(self):
        path = urlparse(self.path).path
        if (
            path.startswith("/game-admin") or path.startswith("/admin")
        ) and not game_admin_authorized(self):
            return send_json(
                self,
                403,
                {
                    "ok": False,
                    "error": "admin_only",
                    "message": "需要管理员权限，请先登录站点",
                },
            )
        if path == "/game-admin/nav.js":
            import os as _os

            nav_path = _os.path.join(_os.path.dirname(__file__), "game_nav.js")
            body = open(nav_path, "rb").read()
            self.send_response(200)
            self.send_header("Content-Type", "application/javascript; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if path == "/health":
            image_config = _get_image_runtime_config()
            return send_json(
                self,
                200,
                {
                    "ok": True,
                    "site_id": SITE_ID,
                    "model": MODEL,
                    "base_url_set": bool(API_BASE_URL),
                    "key_set": bool(API_KEY),
                    "onebot_url": ONEBOT_URL,
                    "image": {
                        "enabled": bool(image_config.get("enabled")),
                        "base_url_set": bool(image_config.get("api_base_url")),
                        "key_set": bool(image_config.get("api_key")),
                        "model": image_config.get("model"),
                        "size": image_config.get("size"),
                        "source": image_config.get("source"),
                    },
                },
            )
        if path == "/game-admin" or path == "/game-admin/":
            body = ADMIN_HTML.encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            self.wfile.write(body)
            return
        if path.startswith("/game-admin/"):
            status, result = handle_admin(path, self)
            return send_json(self, status, result)
        # backward compat: also serve under /admin/
        if path == "/admin" or path == "/admin/":
            body = ADMIN_HTML.encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            self.wfile.write(body)
            return
        if path.startswith("/admin/"):
            status, result = handle_admin(path.replace("/admin/", "/game-admin/"), self)
            return send_json(self, status, result)
        return send_json(self, 404, {"ok": False, "error": "not_found"})

    def do_POST(self):
        path = urlparse(self.path).path
        if (
            path.startswith("/game-admin") or path.startswith("/admin")
        ) and not game_admin_authorized(self):
            return send_json(self, 401, {"ok": False, "error": "unauthorized"})
        if path == "/onebot":
            if not onebot_webhook_authorized(self):
                return send_json(
                    self, 401, {"ok": False, "error": "unauthorized_webhook"}
                )
            return handle_onebot(self)
        if path in ("/telegram", "/tg", "/telegram/webhook", "/tg/webhook"):
            if not telegram_webhook_authorized(self):
                return send_json(
                    self, 401, {"ok": False, "error": "unauthorized_webhook"}
                )
            raw = read_json(self)
            event_key = _telegram_event_key(raw)
            if event_key and _is_duplicate_message(event_key):
                return send_json(
                    self, 200, {"ok": True, "ignored": "duplicate_telegram_update"}
                )
            try:
                result = _tg_notice_event(raw)
                if result is not None:
                    return send_json(
                        self,
                        200,
                        {"ok": True, "telegram_notice_handled": True, "result": result},
                    )
            except Exception as e:
                print(f"[MembershipRisk] TG forward failed: {e}", flush=True)
            result = handle_tg_message(raw)
            return send_json(
                self, 200, result or {"ok": True, "ignored": "telegram_update"}
            )
        if path.startswith("/game-admin/"):
            status, result = handle_admin(path, self)
            return send_json(self, status, result)
        if path.startswith("/admin/"):
            status, result = handle_admin(path.replace("/admin/", "/game-admin/"), self)
            return send_json(self, status, result)
        if not authorized(self):
            return send_json(self, 401, {"ok": False, "error": "unauthorized"})
        if path not in ["/v1/chatops/plan", "/v1/tasks/plan"]:
            return send_json(self, 404, {"ok": False, "error": "not_found"})
        try:
            payload = read_json(self)
            text = str(
                payload.get("text")
                or payload.get("message")
                or payload.get("command")
                or payload.get("raw_command")
                or ""
            )
            source = str(
                payload.get("source") or payload.get("platform") or BOT_PLATFORM or ""
            ).lower()
            if source == "telegram":
                source = "tg"
            room_id = str(
                payload.get("room_id")
                or payload.get("group_id")
                or payload.get("chat_id")
                or ""
            )
            user_id = str(
                payload.get("user_id")
                or payload.get("issuer_id")
                or payload.get("user_external_id")
                or payload.get("issuer_external_id")
                or ""
            )
            username = str(
                payload.get("username") or payload.get("issuer_name") or user_id
            )
            issuer_role = str(payload.get("issuer_role") or "")
            is_admin = _is_source_admin(source, user_id, username)
            admin_source = "configured" if is_admin else "none"
            payload["user_id"] = user_id
            payload["is_admin"] = is_admin
            payload["_trusted_is_admin"] = is_admin
            payload["_trusted_user_id"] = user_id
            payload["_stable_event_key"] = _source_event_key(
                source, room_id or user_id, payload.get("message_id") or ""
            )
            payload["admin_source"] = admin_source
            ad_lookup = _handle_recent_ad_lookup(
                source,
                room_id,
                user_id,
                username,
                text,
                message_id=str(payload.get("message_id") or ""),
                is_admin=is_admin,
            )
            result = (
                ad_lookup
                or handle_with_games(payload)
                or _guard_llm_result(payload, call_model(payload))
            )
            log(
                {
                    "event": "plan",
                    "path": path,
                    "source": payload.get("source"),
                    "role": issuer_role,
                    "risk": result.get("risk"),
                    "approval": result.get("requires_approval"),
                    "user_id": user_id,
                    "message_id": str(payload.get("message_id") or ""),
                    "admin_source": admin_source,
                }
            )
            return send_json(
                self,
                200,
                {
                    "ok": True,
                    "site_id": SITE_ID,
                    "planner": "hermes-adapter",
                    "result": result,
                },
            )
        except Exception as e:
            log(
                {
                    "event": "error",
                    "error": str(e),
                    "trace": traceback.format_exc()[-1200:],
                }
            )
            return send_json(self, 500, {"ok": False, "error": str(e)})


if __name__ == "__main__":
    _server = ThreadingHTTPServer(
        (
            os.environ.get("HERMES_ADAPTER_HOST", "127.0.0.1"),
            int(os.environ.get("HERMES_ADAPTER_PORT", "18181")),
        ),
        Handler,
    )
    try:
        _server.serve_forever()
    finally:
        _stop_burn_scheduler(clear_pending=True)
        _ASYNC_EXECUTOR.shutdown(wait=False, cancel_futures=True)
        _server.server_close()
