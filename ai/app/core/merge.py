"""三路风险融合（与 gateway/internal/engine 的逻辑一致）。

最差两路 7:3 加权，保证单层噪声不会单独触发拦截。
"""
from __future__ import annotations

from typing import Tuple


def merge_risk(a: int, b: int, c: int) -> int:
    arr = sorted([a, b, c], reverse=True)
    hi1, hi2 = arr[0], arr[1]
    merged = (hi1 * 7 + hi2 * 3) // 10
    return max(0, min(100, merged))


def classify(risk: int) -> Tuple[str, str]:
    if risk >= 85:
        return "BLOCK", "block"
    if risk >= 65:
        return "ALERT", "alert"
    if risk >= 35:
        return "WATCH", "alert"
    return "SAFE", "pass"


def guess_registry(num: str) -> str:
    """极简号段映射（生产应替换为字典或运营商接口）。"""
    num = (num or "").strip()
    if num.startswith("+86"):
        return "CN/BJ"
    if num.startswith("+852"):
        return "HK"
    if num.startswith("+886"):
        return "TW"
    if num.startswith("+1"):
        return "US"
    if num.startswith("+855"):
        return "KH"
    if num.startswith("+95"):
        return "MM"
    return "??"
