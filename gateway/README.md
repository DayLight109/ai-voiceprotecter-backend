# Gateway · Sentinel Go API

Go 1.22 + chi v5 网关，负责鉴权、CRUD、SSE、对象存储签名。

## 本地开发

```bash
# 1. 启依赖（pg/redis/minio）
docker compose -f ../deploy/docker-compose.yml up -d postgres redis minio minio-init

# 2. 跑迁移
docker compose -f ../deploy/docker-compose.yml run --rm migrate up

# 3. 本地运行 gateway（热重启用 air）
go run ./cmd/server -addr=:8080
```

## 目录

```
gateway/
├── cmd/server/main.go        入口
├── internal/
│   ├── config/               envconfig
│   ├── api/
│   │   ├── router.go         总路由
│   │   ├── middleware/       中间件链
│   │   └── handlers/         一个文件一个资源
│   ├── auth/                 JWT + bcrypt
│   ├── service/              业务逻辑
│   ├── repo/                 sqlc 生成
│   ├── domain/               实体
│   ├── aiclient/             调 AI 服务
│   ├── feed/                 SSE pub/sub
│   ├── store/                Redis 适配
│   └── storage/              MinIO 封装
├── migrations/               golang-migrate
└── seed/                     测试种子
```

## 代码风格

- handler 文件命名：资源单数（`blacklist.go`），用 chi 子路由组装
- 所有 handler 收 `Deps` 结构，依赖通过构造注入，方便测试
- sqlc 生成代码不手改；改 SQL 后 `make gateway-sqlc`
- 错误返回包：`apperr.Error{Code, Message, HTTP}` → 中间件统一 JSON
