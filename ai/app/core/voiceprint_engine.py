"""声纹合成检测：ONNX 推理。

输入：16 kHz PCM 数组（np.float32）
输出：(synth_prob, f0_jitter, breath_score, regularity)

模型不可用时抛 ModelUnavailable —— 不返回任何伪概率。
"""
from __future__ import annotations

import logging
import os
from typing import Optional, Tuple

from app.core.config import settings
from app.core.whisper_engine import ModelUnavailable  # 复用同一异常类型

log = logging.getLogger(__name__)


class VoiceprintEngine:
    def __init__(self) -> None:
        self._session = None
        self._load_error: Optional[str] = None

    def warmup(self) -> None:
        if not os.path.isfile(settings.voiceprint_model_path):
            self._load_error = f"model file not found: {settings.voiceprint_model_path}"
            log.warning(self._load_error)
            return
        try:
            import onnxruntime as ort  # noqa: PLC0415

            self._session = ort.InferenceSession(
                settings.voiceprint_model_path,
                providers=["CPUExecutionProvider"],
            )
            self._load_error = None
            log.info("voiceprint loaded: %s", settings.voiceprint_model_path)
        except Exception as e:  # noqa: BLE001
            self._load_error = str(e)
            log.warning("voiceprint load failed: %s", e)
            self._session = None

    def infer(self, audio_seconds: float) -> Tuple[float, float, float, float]:
        """返回 (synth_prob, f0_jitter, breath, regularity)。

        模型未加载时抛 ModelUnavailable，由 router 翻成 503。
        真实实现把 self._session.run(input_feed) 的输出做后处理即可。
        """
        if self._session is None:
            raise ModelUnavailable(
                f"voiceprint model not loaded: {self._load_error or 'unknown'}"
            )

        # TODO P3: 真模型替换
        # feats = preprocess_pcm(audio_bytes)         # (1, n_frames, mels)
        # out   = self._session.run(None, {"input": feats})
        # synth, jitter, breath, regularity = postprocess(out)
        # return synth, jitter, breath, regularity
        raise ModelUnavailable("voiceprint inference not implemented; awaiting P3")

    def close(self) -> None:
        self._session = None


voiceprint_engine = VoiceprintEngine()
