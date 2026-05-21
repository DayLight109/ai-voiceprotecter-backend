"""LLM 分发入口。

对外暴露：
    - LLMUnavailable          （异常）
    - async classify_script() （路由到 provider）

按 `settings.llm_provider` 把请求路由到 qwen / openai。
空 transcript 直接短路返回，不算 mock —— 是合法输入。
"""
from __future__ import annotations

import logging
from typing import Any, Dict

from app.core.config import settings
from app.core.llm_base import LLMUnavailable

log = logging.getLogger(__name__)

__all__ = ["LLMUnavailable", "classify_script"]


async def classify_script(transcript: str) -> Dict[str, Any]:
    """返回 { hits: [{category, phrase, weight}], risk: int }。

    transcript 为空 → 空命中（合法）。
    其余情况按 provider 实际调用，失败统一抛 LLMUnavailable。
    """
    text = (transcript or "").strip()
    if not text:
        return {"hits": [], "risk": 0}

    provider = (settings.llm_provider or "qwen").lower()
    if provider == "openai":
        from app.core import openai_client  # noqa: PLC0415
        return await openai_client.classify(text)
    if provider == "qwen":
        from app.core import qwen_client  # noqa: PLC0415
        return await qwen_client.classify(text)
    raise LLMUnavailable(f"unknown LLM_PROVIDER: {provider}")
