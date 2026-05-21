# SENTINEL Server · 声纹捕手后端

> Go 网关 + Python AI 子服务，为 [声纹捕手](../声纹捕手) 前端提供完整的反诈 API。

## 服务拓扑

```
Next.js 前端
      ↓ /api/v1/*
┌──────────────────────────────────────┐
│ gateway · Go 1.22 + chi  (:8080)     │   ← 鉴权 / CRUD / SSE / 文件
└──────┬────────────┬──────────────┬───┘
       │ pgx        │ redis        │ HTTP
       ▼            ▼              ▼
   PostgreSQL    Redis        ai · Python (:8090)
       │                          │ ← Whisper / ONNX / 千问
       ▼                          ▼
   持久化数据                    模型推理
                              ↓
                            MinIO 对象存储
```

## 一键启动

```bash
cp .env.example .env
docker compose -f deploy/docker-compose.yml up -d
make migrate       # 跑数据库迁移
make seed          # 仅注入 1 个 global 租户 + 1 个 sysadmin（无演示数据）
```

骨架阶段所有业务表为空，list 端点返回空集合；非 list 端点返回 `501 NOT_IMPLEMENTED` —— 等待 P1 业务层实现。

启动后：
- gateway → http://localhost:8080
- ai      → http://localhost:8090
- minio   → http://localhost:9001  (用户 / 密码见 .env)
- pg      → localhost:5432

## 健康检查

```bash
curl http://localhost:8080/api/v1/health
curl http://localhost:8090/healthz
```

## 目录速览

```
sentinel-server/
├── gateway/        Go 网关：业务 CRUD + 鉴权 + SSE
├── ai/             Python AI：Whisper + ONNX + 千问
├── deploy/         docker-compose / init.sql / nginx
├── docs/           openapi.yaml / ER 图 / 时序图
├── ARCHITECTURE.md 详细架构
├── API.md          全部接口
├── DATABASE.md     表结构
└── Makefile        常用命令
```

## 文档导航

| 文件 | 内容 |
|---|---|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | 服务拓扑、职责切分、数据流、关键决策 |
| [API.md](./API.md) | 全部 REST 端点（按模块） |
| [DATABASE.md](./DATABASE.md) | 数据库表结构 + 关系 |
| [docs/openapi.yaml](./docs/openapi.yaml) | OpenAPI 契约（前端可生成 client） |
| [gateway/README.md](./gateway/README.md) | Go 网关开发指南 |
| [ai/README.md](./ai/README.md) | Python AI 开发指南 |

## 与前端对接

在 `声纹捕手/.env.local` 写：

```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

重启 `next dev`，前端 `useFeed` / `fetch` 会自动指向本服务。

## 分阶段路线

- **P0 · 骨架（当前）** — 目录 / 容器 / 路由 / 中间件 / 数据库 schema 到位；handler 返回空集合或 501，AI 缺模型返 503，无任何 mock 数据
- P1 — 鉴权 + 黑/白名单完整 CRUD
- P2 — 知识库 + 规则库 + 风控等级
- P3 — Whisper / ONNX / 千问真实推理
- P4 — 录音上传 / 样本审核 / 审计 / 设备管理

## License

内部演示用，不对外提供商业服务。
