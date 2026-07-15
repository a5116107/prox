#!/usr/bin/env python3
"""Import a versioned quiz seed through the prox root-admin API."""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_SEED = ROOT / "seed" / "quiz_bank.legacy.json"


class APIError(RuntimeError):
    pass


class ProxAPI:
    def __init__(self, base_url: str, token: str, user_id: str = "") -> None:
        self.base_url = base_url.rstrip("/")
        self.headers = {
            "Accept": "application/json",
            "Authorization": token
            if token.lower().startswith("bearer ")
            else f"Bearer {token}",
            "Content-Type": "application/json",
            "User-Agent": "prox-quiz-seed/1",
        }
        if user_id:
            self.headers["New-Api-User"] = user_id

    def request(
        self, method: str, path: str, payload: dict[str, Any] | None = None
    ) -> Any:
        data = (
            None
            if payload is None
            else json.dumps(payload, ensure_ascii=False).encode("utf-8")
        )
        request = urllib.request.Request(
            self.base_url + path,
            data=data,
            headers=self.headers,
            method=method,
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                body = response.read().decode("utf-8")
        except urllib.error.HTTPError as error:
            detail = error.read().decode("utf-8", errors="replace")[:1000]
            raise APIError(f"HTTP {error.code} for {path}: {detail}") from error
        except urllib.error.URLError as error:
            raise APIError(f"request failed for {path}: {error.reason}") from error
        try:
            decoded = json.loads(body)
        except json.JSONDecodeError as error:
            raise APIError(f"non-JSON response for {path}: {body[:500]}") from error
        if not isinstance(decoded, dict) or not decoded.get("success"):
            message = (
                decoded.get("message")
                if isinstance(decoded, dict)
                else "invalid API response"
            )
            raise APIError(f"API rejected {path}: {message}")
        return decoded.get("data")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--base-url", default=os.environ.get("PROX_BASE_URL", "http://127.0.0.1:3000")
    )
    parser.add_argument("--token", default=os.environ.get("PROX_ADMIN_TOKEN", ""))
    parser.add_argument("--user-id", default=os.environ.get("PROX_ADMIN_USER_ID", ""))
    parser.add_argument(
        "--site-id", default=os.environ.get("COMMUNITY_SITE_ID", "prox")
    )
    parser.add_argument("--seed", type=Path, default=DEFAULT_SEED)
    parser.add_argument("--bank-code", default="legacy-general")
    parser.add_argument("--bank-name", default="Legacy general knowledge")
    parser.add_argument(
        "--apply", action="store_true", help="write after the mandatory server dry-run"
    )
    parser.add_argument(
        "--publish", action="store_true", help="publish questions and the bank"
    )
    return parser.parse_args()


def load_questions(path: Path) -> list[dict[str, Any]]:
    with path.open("r", encoding="utf-8-sig") as handle:
        document = json.load(handle)
    rows = document.get("questions") if isinstance(document, dict) else None
    if not isinstance(rows, list) or not rows:
        raise ValueError("seed must contain a non-empty questions array")
    questions: list[dict[str, Any]] = []
    for index, row in enumerate(rows, start=1):
        if not isinstance(row, dict):
            raise ValueError(f"question {index} is not an object")
        tags = row.get("tags") if isinstance(row.get("tags"), list) else []
        questions.append(
            {
                "external_key": str(row.get("id") or f"legacy-{index:04d}"),
                "category_code": str(tags[0] if tags else "general"),
                "category_name": str(tags[0] if tags else "General"),
                "prompt": str(row.get("question") or "").strip(),
                "options": row.get("options") or [],
                "correct_index": row.get("answer_index"),
                "explanation": str(row.get("explanation") or ""),
                "difficulty": str(row.get("difficulty") or "normal"),
                "language": str(row.get("language") or "zh-CN"),
                "status": "published",
                "weight": int(row.get("weight") or 100),
                "source": f"seed-v{document.get('version', 1)}",
            }
        )
    return questions


def find_bank(api: ProxAPI, site_id: str, code: str) -> dict[str, Any] | None:
    path = f"/api/ops/quiz/{urllib.parse.quote(site_id, safe='')}/banks"
    data = api.request("GET", path)
    for item in (data or {}).get("items", []):
        bank = item.get("bank") or {}
        if bank.get("code") == code:
            return bank
    return None


def main() -> int:
    args = parse_args()
    if not args.token:
        raise SystemExit("--token or PROX_ADMIN_TOKEN is required")
    questions = load_questions(args.seed.resolve())
    api = ProxAPI(args.base_url, args.token, args.user_id)
    site = urllib.parse.quote(args.site_id, safe="")

    bank = find_bank(api, args.site_id, args.bank_code)
    if bank is None:
        if not args.apply:
            print(
                json.dumps(
                    {
                        "dry_run": True,
                        "bank": "would_create",
                        "questions": len(questions),
                    },
                    ensure_ascii=False,
                )
            )
            return 0
        bank = api.request(
            "POST",
            f"/api/ops/quiz/{site}/banks",
            {
                "code": args.bank_code,
                "name": args.bank_name,
                "description": f"Imported from {args.seed.name}",
                "default_language": "zh-CN",
                "status": "draft",
            },
        )

    bank_id = int(bank["id"])
    import_path = f"/api/ops/quiz/{site}/banks/{bank_id}/import"
    payload = {"dry_run": True, "publish": args.publish, "questions": questions}
    preview = api.request("POST", import_path, payload)
    print(
        json.dumps(
            {"stage": "server_dry_run", "bank_id": bank_id, "result": preview},
            ensure_ascii=False,
        )
    )
    if not args.apply:
        return 0

    payload["dry_run"] = False
    result = api.request("POST", import_path, payload)
    if args.publish:
        api.request("POST", f"/api/ops/quiz/{site}/banks/{bank_id}/publish", {})
    print(
        json.dumps(
            {
                "stage": "applied",
                "bank_id": bank_id,
                "bank_published": args.publish,
                "result": result,
            },
            ensure_ascii=False,
        )
    )
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (APIError, OSError, ValueError) as error:
        print(f"quiz seed import failed: {error}", file=sys.stderr)
        raise SystemExit(1) from error
