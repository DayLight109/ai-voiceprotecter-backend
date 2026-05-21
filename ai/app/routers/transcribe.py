"""POST /v1/transcribe — 把音频转成文字。

模型不可用 → 503 MODEL_UNAVAILABLE
"""
import time

from fastapi import APIRouter, HTTPException

from app.core.whisper_engine import ModelUnavailable, whisper_engine
from app.schemas.analyze import TranscribeRequest, TranscribeResponse

router = APIRouter(tags=["whisper"])


@router.post("/transcribe", response_model=TranscribeResponse)
async def transcribe(req: TranscribeRequest) -> TranscribeResponse:
    start = time.perf_counter()
    try:
        text = await whisper_engine.transcribe(req.audioKey, language=req.language)
    except ModelUnavailable as e:
        raise HTTPException(status_code=503, detail={
            "code": "MODEL_UNAVAILABLE",
            "message": str(e),
        }) from e

    return TranscribeResponse(
        text=text,
        language=req.language or "zh",
        durationMillis=int((time.perf_counter() - start) * 1000),
    )
