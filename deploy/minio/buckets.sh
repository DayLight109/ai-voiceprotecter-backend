#!/usr/bin/env bash
# MinIO 初始化：创建桶 + 配置访问策略
#
# 由 docker-compose 的 minio-init 服务自动执行
set -euo pipefail

ENDPOINT="${MINIO_ENDPOINT:-http://minio:9000}"
ACCESS_KEY="${MINIO_ACCESS_KEY:-sentinel}"
SECRET_KEY="${MINIO_SECRET_KEY:-sentinel123}"

mc alias set local "${ENDPOINT}" "${ACCESS_KEY}" "${SECRET_KEY}"

for bucket in \
    "${MINIO_BUCKET_RECORDINGS:-sentinel-recordings}" \
    "${MINIO_BUCKET_MODELS:-sentinel-models}" \
    "${MINIO_BUCKET_CREDENTIALS:-sentinel-credentials}"; do
    mc mb --ignore-existing "local/${bucket}"
    # 默认私有，只能通过预签名访问
    mc anonymous set none "local/${bucket}" || true
done

echo "✓ buckets ready"
