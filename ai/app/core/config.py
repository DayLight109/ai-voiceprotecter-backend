"""集中读取环境变量（pydantic-settings v2）。

两层 AI 引擎都允许通过 provider 开关在 **本地** / **云端** 之间切换：

* 语音转写 (Whisper)
    - `WHISPER_PROVIDER=local`  → faster-whisper（默认；可加载自训练微调权重）
    - `WHISPER_PROVIDER=openai` → OpenAI Audio API（gpt-4o-mini-transcribe / whisper-1）

* 话术分类 (LLM)
    - `LLM_PROVIDER=qwen`   → 阿里 DashScope 千问（默认）
    - `LLM_PROVIDER=openai` → OpenAI Chat Completions（gpt-4o-mini / 自部署兼容端点）
"""
from typing import List, Literal

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    # ── 服务 ────────────────────────────────
    log_level: str = "INFO"
    cors_allowed_origins: List[str] = Field(default_factory=lambda: ["*"])

    # ── Whisper provider 开关 ──────────────
    whisper_provider: Literal["local", "openai"] = "local"

    # ── Whisper · 本地 (faster-whisper) ────
    # `whisper_model_path` 优先，用于加载自训练 / 微调权重（CTranslate2 目录或
    # HuggingFace repo）；未填则用 `whisper_model_size` 拉官方预训练模型。
    whisper_model_path: str = ""
    whisper_model_size: str = "large-v3"     # tiny / base / small / medium / large-v3
    whisper_device: str = "cpu"              # cpu / cuda
    whisper_compute_type: str = "int8"       # int8 / float16 / float32
    whisper_language: str = "zh"
    whisper_vad_filter: bool = True
    whisper_cache_dir: str = "/app/models/whisper-cache"

    # ── Whisper · OpenAI 云端 ──────────────
    # 走 OpenAI Audio API；填了 base_url 可指向自部署 / Azure / 第三方兼容代理。
    whisper_openai_model: str = "whisper-1"  # whisper-1 | gpt-4o-mini-transcribe | gpt-4o-transcribe

    # ── ONNX 声纹 ──────────────────────────
    voiceprint_model_path: str = "/app/models/voiceguard-v2.6.1.onnx"
    voiceprint_threshold: float = 0.55

    # ── LLM provider 开关 ──────────────────
    llm_provider: Literal["qwen", "openai"] = "qwen"

    # ── DashScope (千问) ───────────────────
    dashscope_api_key: str = ""
    dashscope_endpoint: str = (
        "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"
    )
    qwen_model: str = "qwen-max"
    qwen_temperature: float = 0.2
    qwen_top_p: float = 0.9
    qwen_max_tokens: int = 1024

    # ── OpenAI (Chat + Audio 共享) ─────────
    openai_api_key: str = ""
    openai_base_url: str = ""                # 留空 = 官方 https://api.openai.com/v1
    openai_chat_model: str = "gpt-4o-mini"
    openai_temperature: float = 0.2
    openai_top_p: float = 0.9
    openai_max_tokens: int = 1024

    # ── 公共系统提示词（两种 LLM provider 都用同一份） ──
    llm_system_prompt: str = (
        "你是一名反诈通话分析专家。请判定通话内容是否构成电信诈骗，"
        "并按 5 类标签 + 置信度输出 JSON。"
    )


settings = Settings()
