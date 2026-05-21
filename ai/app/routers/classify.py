"""POST /v1/classify — 把转写文本送入千问做 5 类话术判定。

LLM 未配置 / 调用失败 → 503 LLM_UNAVAILABLE
"""
from fastapi import APIRouter, HTTPException

from app.core.llm_client import LLMUnavailable, classify_script
from app.schemas.analyze import ClassifyRequest, ScriptVerdict

router = APIRouter(tags=["script"])


@router.post("/classify", response_model=ScriptVerdict)
async def classify(req: ClassifyRequest) -> ScriptVerdict:
    try:
        result = await classify_script(req.transcript)
    except LLMUnavailable as e:
        raise HTTPException(status_code=503, detail={
            "code": "LLM_UNAVAILABLE",
            "message": str(e),
        }) from e
    return ScriptVerdict(**result)
