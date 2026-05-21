# ONNX 模型放置说明

本目录 **不提交真实模型文件**（`*.onnx` 已加入 `.gitignore`）。

## 部署时如何放置

```
models/
├── voiceguard-v2.6.1.onnx     声纹合成检测主模型（约 184 MB）
└── whisper-cache/             faster-whisper 自动下载缓存
```

## 自动下载示例

```python
from faster_whisper import WhisperModel
WhisperModel("large-v3", device="cpu", compute_type="int8",
             download_root="/app/models/whisper-cache")
```

## 替换模型版本

通过 gateway `/api/v1/voice-models` 上传后，下载新文件到本目录并重启 ai 容器。
未来可改为 ai 启动时主动拉 MinIO（见 `core/voiceprint_engine.py` 的 `_load`）。
