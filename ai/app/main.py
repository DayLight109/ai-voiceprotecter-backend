"""Sentinel AI · FastAPI 入口。

路由分模块挂载：
  /v1/transcribe   Whisper 转写
  /v1/voiceprint   ONNX 声纹合成检测
  /v1/classify     千问 LLM 话术判定
  /v1/analyze      一站式聚合
  /healthz         健康检查
"""
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.core.config import settings
from app.routers import analyze, classify, healthz, transcribe, voiceprint


@asynccontextmanager
async def lifespan(app: FastAPI):  # noqa: ARG001
    # 启动：预热模型（让首次请求不要慢）
    from app.core.voiceprint_engine import voiceprint_engine
    from app.core.whisper_engine import whisper_engine

    voiceprint_engine.warmup()
    whisper_engine.warmup()
    yield
    # 关闭：释放显存等
    voiceprint_engine.close()
    whisper_engine.close()


app = FastAPI(
    title="Sentinel AI",
    version="0.1.0",
    description="语音反诈三层推理服务",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_allowed_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(healthz.router)
app.include_router(transcribe.router, prefix="/v1")
app.include_router(voiceprint.router, prefix="/v1")
app.include_router(classify.router, prefix="/v1")
app.include_router(analyze.router, prefix="/v1")
