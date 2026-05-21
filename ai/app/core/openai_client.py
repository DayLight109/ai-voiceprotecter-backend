"""OpenAI Chat 客户端。

可指向官方 / Azure / 第三方兼容代理（通过 OPENAI_BASE_URL）。
未配置 OPENAI_API_KEY 直接抛 LLMUnavailable。
"""
from __future__ import annotations

import logging
from typing import Any, Dict

import httpx

from app.core.config import settings
from app.core.llm_base import (
    LLMUnavailable,
    build_user_instruction,
    coerce_hits,
    extract_json_block,
    score_from_hits,
)

log = logging.getLogger(__name__)


def _api_key_configured() -> bool:
    k = (settings.openai_api_key or "").strip()
    return bool(k) and not k.startswith("sk-x")


def _chat_endpoint() -> str:
    base = (settings.openai_base_url or "https://api.openai.com/v1").rstrip("/")
    return f"{base}/chat/completions"


async def classify(transcript: str) -> Dict[str, Any]:
    if not _api_key_configured():
        raise LLMUnavailable("OPENAI_API_KEY not configured")

    try:
        async with httpx.AsyncClient(timeout=20.0) as client:
            resp = await client.post(
                _chat_endpoint(),
                headers={
                    "Authorization": f"Bearer {settings.openai_api_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": settings.openai_chat_model,
                    "messages": [
                        {"role": "system", "content": build_user_instruction(settings.llm_system_prompt)},
                        {"role": "user", "content": transcript},
                    ],
                    "temperature": settings.openai_temperature,
                    "top_p": settings.openai_top_p,
                    "max_tokens": settings.openai_max_tokens,
                    "response_format": {"type": "json_object"},
                },
            )
            resp.raise_for_status()
            data = resp.json()
    except httpx.HTTPError as e:
        log.warning("openai http error: %s", e)
        raise LLMUnavailable(f"openai request failed: {e}") from e

    content = _extract_assistant_text(data)
    hits = coerce_hits(extract_json_block(content))
    return score_from_hits(hits)


def _extract_assistant_text(data: Dict[str, Any]) -> str:
    choices = data.get("choices") or []
    if not choices:
        return ""
    msg = choices[0].get("message") or {}
    c = msg.get("content")
    if isinstance(c, str):
        return c
    if isinstance(c, list):
        # Some compatible proxies return list-of-parts
        return "".join(seg.get("text", "") for seg in c if isinstance(seg, dict))
    return ""
