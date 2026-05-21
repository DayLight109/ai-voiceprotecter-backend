"""阿里 DashScope · 千问 客户端。

未配置 / 调用失败 → LLMUnavailable。
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
    k = (settings.dashscope_api_key or "").strip()
    return bool(k) and not k.startswith("sk-x")


async def classify(transcript: str) -> Dict[str, Any]:
    if not _api_key_configured():
        raise LLMUnavailable("DASHSCOPE_API_KEY not configured")

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            resp = await client.post(
                settings.dashscope_endpoint,
                headers={
                    "Authorization": f"Bearer {settings.dashscope_api_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": settings.qwen_model,
                    "input": {
                        "messages": [
                            {"role": "system", "content": build_user_instruction(settings.llm_system_prompt)},
                            {"role": "user", "content": transcript},
                        ]
                    },
                    "parameters": {
                        "temperature": settings.qwen_temperature,
                        "top_p": settings.qwen_top_p,
                        "max_tokens": settings.qwen_max_tokens,
                        "result_format": "message",
                    },
                },
            )
            resp.raise_for_status()
            data = resp.json()
    except httpx.HTTPError as e:
        log.warning("qwen http error: %s", e)
        raise LLMUnavailable(f"qwen request failed: {e}") from e

    content = _extract_assistant_text(data)
    hits = coerce_hits(extract_json_block(content))
    return score_from_hits(hits)


def _extract_assistant_text(data: Dict[str, Any]) -> str:
    """兼容 DashScope 的两种返回 schema。"""
    output = data.get("output") or {}
    if isinstance(output.get("choices"), list) and output["choices"]:
        msg = output["choices"][0].get("message") or {}
        c = msg.get("content")
        if isinstance(c, str):
            return c
        if isinstance(c, list):
            return "".join(seg.get("text", "") for seg in c if isinstance(seg, dict))
    if isinstance(output.get("text"), str):
        return output["text"]
    return ""
