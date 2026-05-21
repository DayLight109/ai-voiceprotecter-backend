"""POST /v1/analyze — 并发跑三层，融合后返回完整判决。

是 gateway 的主要入口。

设计：
  - trace 层不依赖任何模型，纯规则（号段 vs 信令源），始终可用
  - voice / script 层模型不可用时分别抛 ModelUnavailable / LLMUnavailable
  - 任一关键层缺失 → 503 MODEL_UNAVAILABLE，附 details 表明缺哪一层
"""
import asyncio
import time
from datetime import datetime, timezone

from fastapi import APIRouter, HTTPException

from app.core.llm_client import LLMUnavailable, classify_script
from app.core.merge import classify, guess_registry, merge_risk
from app.core.voiceprint_engine import voiceprint_engine
from app.core.whisper_engine import ModelUnavailable
from app.schemas.analyze import (
    AnalyzeRequest,
    AnalyzeResponse,
    ScriptHit,
    ScriptVerdict,
    TraceVerdict,
    VoiceVerdict,
)

router = APIRouter(tags=["analyze"])


@router.post("/analyze", response_model=AnalyzeResponse)
async def analyze(req: AnalyzeRequest) -> AnalyzeResponse:
    start = time.perf_counter()

    trace_t = asyncio.create_task(_trace(req))
    voice_t = asyncio.create_task(_voice(req))
    script_t = asyncio.create_task(_script(req))

    trace, voice_res, script_res = await asyncio.gather(
        trace_t, voice_t, script_t, return_exceptions=True
    )

    # 收集底层引擎错误，决定是否退化或失败
    failures = []
    if isinstance(voice_res, ModelUnavailable):
        failures.append({"layer": "voiceprint", "reason": str(voice_res)})
        voice = None
    elif isinstance(voice_res, Exception):
        raise voice_res
    else:
        voice = voice_res

    if isinstance(script_res, LLMUnavailable):
        failures.append({"layer": "script", "reason": str(script_res)})
        script = None
    elif isinstance(script_res, Exception):
        raise script_res
    else:
        script = script_res

    if isinstance(trace, Exception):
        raise trace

    # 至少 trace 一定可用；voice / script 缺失 → 503 上抛
    if failures:
        raise HTTPException(
            status_code=503,
            detail={"code": "MODEL_UNAVAILABLE", "failures": failures},
        )

    risk = merge_risk(trace.risk, voice.risk, script.risk)
    level, action = classify(risk)

    return AnalyzeResponse(
        callId=req.callId,
        ts=datetime.now(timezone.utc).isoformat(),
        trace=trace,
        voiceprint=voice,
        script=script,
        riskScore=risk,
        riskLevel=level,
        action=action,
        latencyMillis=int((time.perf_counter() - start) * 1000),
    )


async def _trace(req: AnalyzeRequest) -> TraceVerdict:
    """L1 溯源 — 不依赖模型，纯规则比对。"""
    registry = guess_registry(req.shownNumber)
    origin = (req.signalOriginCC or "").upper()
    if not origin:
        # 没有信令源信息时，无法判断匹配性
        return TraceVerdict(
            shownRegistry=registry,
            actualOrigin="",
            mismatch=False,
            hopCount=0,
            risk=0,
            note="signal origin not provided",
        )
    mismatch = not registry.startswith(origin)
    risk = 86 if mismatch else 6
    note = (
        f"signal origin {origin} ≠ declared registry {registry}"
        if mismatch
        else "registry matches signal route"
    )
    return TraceVerdict(
        shownRegistry=registry,
        actualOrigin=origin,
        mismatch=mismatch,
        hopCount=5 if mismatch else 2,
        risk=risk,
        note=note,
    )


async def _voice(req: AnalyzeRequest) -> VoiceVerdict:
    """L2 声纹 — ONNX 推理；模型不可用抛 ModelUnavailable。"""
    synth, jitter, breath, regularity = voiceprint_engine.infer(req.audioSeconds)
    risk = int(synth * 100)
    if synth >= 0.85:
        verdict = "SYNTH"
    elif synth >= 0.55:
        verdict = "SUSPECT"
    else:
        verdict = "HUMAN"
    return VoiceVerdict(
        synthProbability=round(synth, 2),
        f0Jitter=round(jitter, 3),
        breathScore=round(breath, 2),
        regularity=round(regularity, 2),
        risk=risk,
        verdict=verdict,
    )


async def _script(req: AnalyzeRequest) -> ScriptVerdict:
    """L3 话术 — 千问 LLM；未配置 / 失败抛 LLMUnavailable。"""
    result = await classify_script(req.transcriptHint or "")
    return ScriptVerdict(
        hits=[ScriptHit(**h) for h in result.get("hits", [])],
        risk=result.get("risk", 0),
    )
