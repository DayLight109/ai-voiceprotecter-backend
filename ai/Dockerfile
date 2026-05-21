# ───── build ─────
FROM python:3.11-slim AS builder

ENV PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /build
COPY requirements.txt .
RUN pip install --user -r requirements.txt

# ───── runtime ───
FROM python:3.11-slim

# ffmpeg 用于 faster-whisper 解码
RUN apt-get update && apt-get install -y --no-install-recommends \
        ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# 复制依赖
COPY --from=builder /root/.local /root/.local
ENV PATH=/root/.local/bin:$PATH \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

WORKDIR /app
COPY app ./app
COPY models ./models

EXPOSE 8090
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8090"]
