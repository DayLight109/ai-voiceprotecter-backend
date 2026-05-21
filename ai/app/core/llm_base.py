"""LLM 公共类型 / 工具。

所有 provider 输出统一为 {hits:[{category, phrase, weight}], risk:int}。
"""
from __future__ import annotations

import json
import re
from typing import Any, Dict, List, Optional


class LLMUnavailable(RuntimeError):
    """LLM 未配置 / 调用失败。由 router 翻成 503。"""


# 5 类标签（与系统提示词约定，强约束模型只能从这 5 类里选）
ALLOWED_CATEGORIES = {
    "切断外部联系", "制造紧迫感", "引导转账", "假冒权威", "索要敏感信息",
}


def build_user_instruction(system_prompt: str) -> str:
    """拼出强约束 JSON schema 描述，附在 system 消息后面。"""
    return system_prompt + (
        "\n请按 JSON 输出，schema: "
        "{\"hits\":[{\"category\":\"...\",\"phrase\":\"...\",\"weight\":0-100}]}。"
        " category 必须从这 5 类中选："
        + "、".join(sorted(ALLOWED_CATEGORIES))
        + "。如无任何命中返回 {\"hits\":[]}。"
    )


_JSON_BLOCK_RE = re.compile(r"\{[\s\S]*\}")


def extract_json_block(text: str) -> Optional[Dict[str, Any]]:
    if not text:
        return None
    m = _JSON_BLOCK_RE.search(text)
    if not m:
        return None
    try:
        return json.loads(m.group(0))
    except json.JSONDecodeError:
        return None


def coerce_hits(payload: Optional[Dict[str, Any]]) -> List[Dict[str, Any]]:
    if not payload:
        return []
    raw_hits = payload.get("hits") or []
    out: List[Dict[str, Any]] = []
    for h in raw_hits:
        if not isinstance(h, dict):
            continue
        category = str(h.get("category", "")).strip()
        if category not in ALLOWED_CATEGORIES:
            continue
        try:
            weight = int(h.get("weight", 0))
        except (TypeError, ValueError):
            continue
        weight = max(0, min(100, weight))
        out.append({
            "category": category,
            "phrase": str(h.get("phrase", "")),
            "weight": weight,
        })
    out.sort(key=lambda h: h["weight"], reverse=True)
    return out


def score_from_hits(hits: List[Dict[str, Any]]) -> Dict[str, Any]:
    """统一打分：取最大权重，叠加其它命中的 1/3 作为补充。"""
    if not hits:
        return {"hits": [], "risk": 0}
    max_w = max(h["weight"] for h in hits)
    extras = sum(h["weight"] for h in hits) - max_w
    risk = min(99, max_w + extras // 3)
    return {"hits": hits, "risk": risk}
