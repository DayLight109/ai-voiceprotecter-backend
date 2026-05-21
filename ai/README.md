# AI Service · Sentinel

> FastAPI + faster-whisper + onnxruntime + (DashScope 千问 / OpenAI)

## 职责

- **/v1/transcribe** — 音频 → 文本（Whisper）
- **/v1/voiceprint** — 合成概率 + F0 抖动 + 呼吸特征（ONNX）
- **/v1/classify** — 文本 → 5 类话术 + 置信度（LLM）
- **/v1/analyze** — 一站式聚合，并发跑三层，融合后返回 SAFE/WATCH/ALERT/BLOCK

**严守边界**：

- 无业务逻辑、无数据库依赖
- 模型路径 & API key 全部走环境变量
- 由 gateway 注入完整上下文（音频 key、号码、用户偏好）

---

## Provider 切换（两层 AI 都支持本地 / 云端）

### 语音转写 · Whisper

| `WHISPER_PROVIDER` | 实现 | 何时用 |
|---|---|---|
| `local` | faster-whisper (CTranslate2) | 默认；离线 / 私有部署 / 微调模型 |
| `openai` | OpenAI Audio API | 不想自己部署 GPU 时 |

**自训练 / 微调权重**：把 CTranslate2 目录或 HuggingFace repo id 填到 `WHISPER_MODEL_PATH` 即覆盖 `WHISPER_MODEL_SIZE`。

```bash
# .env 片段
WHISPER_PROVIDER=local
WHISPER_MODEL_PATH=/app/models/whisper-zh-finetuned   # 微调权重
WHISPER_DEVICE=cuda
WHISPER_COMPUTE_TYPE=float16
```

或者切到云端：

```bash
WHISPER_PROVIDER=openai
WHISPER_OPENAI_MODEL=gpt-4o-mini-transcribe
OPENAI_API_KEY=sk-...
OPENAI_BASE_URL=                          # 留空走官方；填 Azure / 代理 / vLLM
```

### 话术分类 · LLM

| `LLM_PROVIDER` | 实现 |
|---|---|
| `qwen` | 阿里 DashScope · qwen-max（默认） |
| `openai` | OpenAI Chat Completions（`response_format=json_object`） |

两种 provider 共享同一份 `LLM_SYSTEM_PROMPT`，输出 schema 完全一致：
`{"hits":[{"category","phrase","weight":0-100}], "risk":0-99}`。

```bash
# 用 OpenAI
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
OPENAI_CHAT_MODEL=gpt-4o-mini
```

---

## 微调 Whisper 工作流（本地 provider）

1. **准备语料**：`{audio_path, transcript}` 配对，建议 ≥ 50h 中文反诈通话语料。
2. **HuggingFace 微调**：用 `transformers` + `Seq2SeqTrainer` 在 `openai/whisper-large-v3` 上 LoRA / 全参微调。
3. **导出 CTranslate2**：

   ```bash
   pip install ctranslate2 transformers
   ct2-transformers-converter \
       --model ./whisper-zh-finetuned-hf \
       --output_dir ./models/whisper-zh-finetuned \
       --quantization int8
   ```

4. **指向新权重**：`WHISPER_MODEL_PATH=/app/models/whisper-zh-finetuned`，重启容器即生效。
5. （可选）放进 MinIO `sentinel-models` 桶，让 gateway 提供版本切换 API（`/voice-models/{id}/activate`）。

---

## 失败语义（重要）

**没有 mock 兜底**。任一引擎不可用都明示给调用方：

| 情况 | HTTP | code |
|---|---|---|
| Whisper 本地未加载 / OpenAI key 未配 | 503 | `MODEL_UNAVAILABLE` |
| ONNX 模型未加载 | 503 | `MODEL_UNAVAILABLE` |
| `DASHSCOPE_API_KEY` / `OPENAI_API_KEY` 缺失 | 503 | `LLM_UNAVAILABLE` |
| `/v1/analyze` 部分层缺失 | 503 | `MODEL_UNAVAILABLE` + `failures:[{layer,reason}]` |
| 输入校验失败 | 422 | (FastAPI 默认) |

L1 溯源（号段 vs 信令源）是纯规则，始终可用；voice / script 必须靠真模型。

---

## 本地开发

```bash
pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8090 --reload
open http://localhost:8090/docs
```

## 目录

```
ai/
├── app/
│   ├── main.py
│   ├── routers/                每个文件一个端点
│   └── core/
│       ├── whisper_engine.py   local + openai 分发
│       ├── voiceprint_engine.py
│       ├── llm_client.py       provider 分发 facade
│       ├── llm_base.py         共用类型 / hits 解析 / 评分
│       ├── qwen_client.py      DashScope 实现
│       └── openai_client.py    OpenAI 实现
├── models/                     模型挂载位置 (gitignore)
└── tests/
```

## 部署

容器基于 `python:3.11-slim`，`models/` 用 volume 挂入。
faster-whisper 首次启动自动从 HuggingFace 下载；生产建议预下载到 `models/whisper-cache`
或预先准备好微调权重指向 `WHISPER_MODEL_PATH`。
