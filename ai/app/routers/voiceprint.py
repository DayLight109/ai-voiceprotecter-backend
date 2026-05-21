"""POST /v1/voiceprint — 声纹合成检测。

模型不可用 → 503 MODEL_UNAVAILABLE
"""
from fastapi import APIRouter, HTTPException

from app.core.voiceprint_engine import voiceprint_engine
from app.core.whisper_engine import ModelUnavailable
from app.schemas.analyze import VoiceVerdict, VoiceprintRequest

router = APIRouter(tags=["voiceprint"])


@router.post("/voiceprint", response_model=VoiceVerdict)
def voiceprint(req: VoiceprintRequest) -> VoiceVerdict:
    try:
        synth, jitter, breath, regularity = voiceprint_engine.infer(req.audioSeconds)
    except ModelUnavailable as e:
        raise HTTPException(status_code=503, detail={
            "code": "MODEL_UNAVAILABLE",
            "message": str(e),
        }) from e

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
