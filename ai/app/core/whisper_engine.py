"""Whisper 转写引擎封装。

设计原则：
  · 无 mock 兜底——模型不可用直接抛 ModelUnavailable，由 router 翻 503。
  · 双 provider：
      WHISPER_PROVIDER=local  → faster-whisper (CTranslate2)
                                 支持自训练 / 微调权重：WHISPER_MODEL_PATH 优先
                                 否则 WHISPER_MODEL_SIZE 拉官方预训练
      WHISPER_PROVIDER=openai → OpenAI Audio Transcriptions API
                                 OPENAI_BASE_URL 可指向 Azure / 自部署兼容代理
"""
from __future__ import annotations

import logging
import os
from typing import Optional

import httpx

from app.core.config import settings

log = logging.getLogger(__name__)


class ModelUnavailable(RuntimeError):
    """模型未加载 / 加载失败。由 router 翻 503。"""


class WhisperEngine:
    """根据 provider 分发本地 / 云端转写。状态机仅对本地有意义。"""

    def __init__(self) -> None:
        self._model = None
        self._load_error: Optional[str] = None
        self._loaded_target: Optional[str] = None  # 实际加载的路径或 size

    # ── lifecycle ──────────────────────────────────────────────────────
    def warmup(self) -> None:
        """启动时预热。仅本地 provider 真正加载模型；云端只校验 key。"""
        try:
            if settings.whisper_provider == "openai":
                self._check_openai_configured()
                log.info("whisper provider=openai, model=%s", settings.whisper_openai_model)
                return
            self._lazy_load_local()
            log.info("whisper warmed up: %s", self._loaded_target)
        except Exception as e:  # noqa: BLE001
            self._load_error = str(e)
            log.warning("whisper warmup failed: %s", e)

    def close(self) -> None:
        self._model = None

    # ── public ─────────────────────────────────────────────────────────
    async def transcribe(self, audio_path: str, *, language: Optional[str] = None) -> str:
        """转写音频。失败统一抛 ModelUnavailable。"""
        lang = language or settings.whisper_language
        provider = (settings.whisper_provider or "local").lower()
        if provider == "openai":
            return await self._transcribe_openai(audio_path, lang)
        if provider == "local":
            return self._transcribe_local(audio_path, lang)
        raise ModelUnavailable(f"unknown WHISPER_PROVIDER: {provider}")

    # ── local (faster-whisper) ─────────────────────────────────────────
    def _resolve_local_target(self) -> str:
        """优先用户指定的微调权重，其次官方 size。"""
        path = (settings.whisper_model_path or "").strip()
        return path if path else settings.whisper_model_size

    def _lazy_load_local(self) -> None:
        if self._model is not None:
            return
        try:
            from faster_whisper import WhisperModel  # noqa: PLC0415

            target = self._resolve_local_target()
            self._model = WhisperModel(
                target,
                device=settings.whisper_device,
                compute_type=settings.whisper_compute_type,
                download_root=settings.whisper_cache_dir,
            )
            self._loaded_target = target
            self._load_error = None
        except Exception as e:  # noqa: BLE001
            self._load_error = str(e)
            self._model = None
            raise

    def _transcribe_local(self, audio_path: str, language: str) -> str:
        if self._model is None:
            try:
                self._lazy_load_local()
            except Exception as e:  # noqa: BLE001
                raise ModelUnavailable(
                    f"whisper model not loaded: {self._load_error or e}"
                ) from e
        segments, _info = self._model.transcribe(
            audio_path,
            language=language,
            vad_filter=settings.whisper_vad_filter,
            beam_size=5,
        )
        return "".join(seg.text for seg in segments).strip()

    # ── openai cloud ──────────────────────────────────────────────────
    def _check_openai_configured(self) -> None:
        k = (settings.openai_api_key or "").strip()
        if not k or k.startswith("sk-x"):
            raise ModelUnavailable("OPENAI_API_KEY not configured")

    def _openai_endpoint(self) -> str:
        base = (settings.openai_base_url or "https://api.openai.com/v1").rstrip("/")
        return f"{base}/audio/transcriptions"

    async def _transcribe_openai(self, audio_path: str, language: str) -> str:
        try:
            self._check_openai_configured()
        except ModelUnavailable:
            raise
        if not os.path.isfile(audio_path):
            raise ModelUnavailable(f"audio file not found: {audio_path}")

        try:
            with open(audio_path, "rb") as fh:
                files = {"file": (os.path.basename(audio_path), fh.read())}
            data = {
                "model": settings.whisper_openai_model,
                "language": language,
                "response_format": "text",
            }
            async with httpx.AsyncClient(timeout=60.0) as client:
                resp = await client.post(
                    self._openai_endpoint(),
                    headers={"Authorization": f"Bearer {settings.openai_api_key}"},
                    data=data,
                    files=files,
                )
                resp.raise_for_status()
                # response_format=text → 直接是 plain text body
                return resp.text.strip()
        except httpx.HTTPError as e:
            log.warning("openai transcribe http error: %s", e)
            raise ModelUnavailable(f"openai transcribe failed: {e}") from e


whisper_engine = WhisperEngine()
