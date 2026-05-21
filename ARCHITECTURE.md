# 架构说明

## 1. 总体拓扑

```
                ┌─────────────────────────────┐
                │  Next.js 前端 · sentinel    │
                └──────────────┬──────────────┘
                               │ /api/v1/*
                               ▼
┌───────────────────────────────────────────────────────────────┐
│   gateway · Go 1.22 + chi v5                                  │
│   ─────────────────────────────────────────────────────────── │
│   middleware:                                                 │
│     RequestID → Logging → Recover → CORS → RateLimit          │
│           → Auth(JWT) → RBAC → Tenant → Audit                 │
│   handlers (按资源拆分，21 个文件)                              │
│   service (业务逻辑)                                           │
│   repo (sqlc 生成的类型安全 SQL)                                │
└─────┬───────────────┬────────────────┬────────────────────────┘
      │ pgx/v5        │ go-redis       │ HTTP / keep-alive
      ▼               ▼                ▼
┌────────────┐  ┌────────────┐  ┌─────────────────────────────┐
│ PostgreSQL │  │   Redis    │  │  ai · FastAPI + uvicorn      │
│ 主存储      │  │ 会话/缓存/  │  │  ────────────────────────── │
│            │  │ 限流/pub-  │  │  /transcribe → faster-whisper│
│            │  │ sub        │  │  /voiceprint → onnxruntime  │
└────────────┘  └────────────┘  │  /classify   → DashScope    │
                                │  /analyze    (聚合三层)      │
                                └─────────┬───────────────────┘
                                          │
                                          ▼ minio-go
                                   ┌────────────────┐
                                   │     MinIO      │
                                   │ 录音/模型/证件  │
                                   └────────────────┘
```

## 2. 职责切分

| 服务 | 内部端口 | 暴露端口 | 关心的事 |
|---|---|---|---|
| **gateway** | 8080 | 8080 | 鉴权、业务 CRUD、SSE、对象存储签名、调用 AI |
| **ai** | 8090 | 仅内网 | 模型推理 + LLM 调用，**不碰持久化** |
| postgres | 5432 | — | 主数据 |
| redis | 6379 | — | 会话黑名单 / 热点缓存 / 限流计数 |
| minio | 9000 / 9001 | 9001 控制台 | 录音、ONNX、身份证 |

## 3. 数据流：一次实时通话分析

```
前端                  gateway                    ai                 第三方
  │                      │                       │                    │
  ├─POST /analyze────────►                       │                    │
  │  +jwt                ├─审鉴 + RBAC           │                    │
  │                      ├─读热缓 blacklist      │                    │
  │                      ├─POST /v1/analyze─────►│                    │
  │                      │                       ├─whisper.transcribe │
  │                      │                       ├─onnx.voiceprint    │
  │                      │                       ├─dashscope.classify─►千问
  │                      │                       │◄───────────────────┤
  │                      │                       ├─merge (7:3 加权)    │
  │                      │◄──Verdict─────────────┤                    │
  │                      ├─写 call_logs (async)  │                    │
  │                      ├─pub feed (SSE)        │                    │
  │◄─Verdict─────────────┤                       │                    │
```

## 4. 鉴权与多租户

- **JWT 双 token**：access (15 min) + refresh (7 d)。refresh 的 hash 入 `sessions`，黑名单走 Redis。
- **RBAC**：5 角色 `family | biz | family_admin | admin | sysadmin`。chi 路由用装饰器 `r.With(middleware.RequireRole("sysadmin")).Post("/rules", ...)`。
- **Tenant 隔离**：除 sysadmin 外，所有查询自动追加 `WHERE tenant_id = $jwt.tenant_id`。sqlc 生成的方法都带 `tenantID` 参数。
- **审计**：写中间件包装所有 `POST/PUT/DELETE`，落 `audit_logs`。

## 5. 实时事件流（warroom）

- 沿用现有 `backend/internal/feed/hub.go` 的 in-memory 扇出。
- `GET /api/v1/feed/stream` 输出 `text/event-stream`，前端 `EventSource` 直连。
- 上线后可换 Redis Pub/Sub 多实例共享。

## 6. 关键技术决策

| 决策 | 选择 | 理由 |
|---|---|---|
| Go ORM | sqlc | 类型安全、零反射、SQL 即真理 |
| 迁移工具 | golang-migrate | 版本化 + 双向迁移 + Docker 支持 |
| 鉴权 | JWT + refresh | 无状态网关好横向扩展，refresh 给"主动登出" |
| 服务间通信 | HTTP/JSON | 排障容易，调用频次不高（每次通话一次） |
| 文件上传 | MinIO 预签名直传 | 大文件不走网关，省带宽 |
| Python 框架 | FastAPI | 类型注解 + 自动 OpenAPI |
| Whisper 实现 | faster-whisper | 比官方快 4×，CPU 可跑 |
| 风险融合 | 三路最差两路 7:3 | 单层噪声不会触发拦截，沿用现有 engine.mergeRisk |

## 7. 部署模式

- **本地开发**：`docker compose -f deploy/docker-compose.yml up`
- **预发布**：同上加 `--profile staging`，附 nginx 反代
- **生产**：建议
  - gateway / ai 容器化 + k8s
  - postgres → 云 RDS
  - redis → 云 Memorystore
  - minio → 切 S3 / OSS

## 8. 与现有 `声纹捕手/backend/` 的关系

- 现有 backend 是 warroom 的早期原型，**保留不动**。
- 本仓库 `gateway/internal/feed/` 复刻它的 `hub.go` 思路并扩展；`/stats /defcon /feed/stream` 协议**完全兼容**，前端 warroom 切换 `NEXT_PUBLIC_API_URL` 后立刻能跑。
- 现有 `engine/` 三层逻辑在 `ai/app/core/` 用 Python 重写（趋于真实模型）。
