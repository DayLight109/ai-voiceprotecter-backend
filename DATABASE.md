# 数据库设计

> PostgreSQL 16，所有表 `id` 默认 `text`（用 `${prefix}-${ulid}` 便于跨服务对账）

## 表清单

```
                ┌─────────┐         ┌──────────────────────┐
                │ tenants │◄────────│ users                │
                └─────────┘         │  role: family|biz|   │
                     ▲              │       family_admin|  │
                     │              │       admin|sysadmin │
       ┌─────────────┼──────────────┴──┬──────────┬────────┘
       │             │                 │          │
       ▼             ▼                 ▼          ▼
  blacklist     whitelist        identity_   sessions
  call_logs     recordings        credentials
  recording_    risk_level_      ─────────────────
   policy        rules / state   knowledge_articles  (全局)
  permissions   appeals          scam_rules           (全局)
  devices       admin_           voice_models         (全局)
                applications     voice_samples        (全局)
                                 agent_config         (全局)
                                 samples              (全局)
                                 audit_logs           (全局)
```

## 1. tenants

```sql
CREATE TABLE tenants (
  id          text PRIMARY KEY,
  kind        text NOT NULL CHECK (kind IN ('family','enterprise','global')),
  name        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
```

## 2. users

```sql
CREATE TABLE users (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name            text NOT NULL,
  phone           text UNIQUE,
  id_card_hash    text,                    -- SHA256 大写
  email           text,
  password_hash   text NOT NULL,           -- bcrypt cost=12
  role            text NOT NULL CHECK (role IN ('family','biz','family_admin','admin','sysadmin')),
  status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active','review','suspended')),
  dept            text,                    -- 家庭：亲属关系 / 企业：部门
  last_login_at   timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_tenant ON users(tenant_id);
CREATE INDEX idx_users_phone ON users(phone);
```

## 3. identity_credentials

```sql
CREATE TABLE identity_credentials (
  id           text PRIMARY KEY,
  user_id      text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('phone','id_card','passport','military','hk_mo')),
  value_hash   text,
  verified     boolean NOT NULL DEFAULT false,
  verified_at  timestamptz,
  UNIQUE(user_id, kind)
);
```

## 4. sessions

```sql
CREATE TABLE sessions (
  jti                  text PRIMARY KEY,
  user_id              text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_token_hash   text NOT NULL,
  expires_at           timestamptz NOT NULL,
  revoked              boolean NOT NULL DEFAULT false,
  user_agent           text,
  ip                   inet,
  created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
```

## 5. blacklist / whitelist

```sql
CREATE TABLE blacklist (
  id          text PRIMARY KEY,
  tenant_id   text REFERENCES tenants(id) ON DELETE CASCADE,  -- NULL = 全局
  number      text NOT NULL,
  reason      text,
  category    text NOT NULL CHECK (category IN ('AI合成','话术诈骗','号码伪冒','其他')),
  risk        int  NOT NULL CHECK (risk BETWEEN 0 AND 100),
  source      text NOT NULL CHECK (source IN ('本地','云端','手动')),
  created_by  text REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_blacklist_tenant_number ON blacklist(tenant_id, number);
CREATE INDEX idx_blacklist_risk ON blacklist(risk DESC);

CREATE TABLE whitelist (
  id          text PRIMARY KEY,
  tenant_id   text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  number      text NOT NULL,
  name        text,
  relation    text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, number)
);
```

## 6. knowledge_articles (全局)

```sql
CREATE TABLE knowledge_articles (
  id          text PRIMARY KEY,
  title       text NOT NULL,
  category    text NOT NULL CHECK (category IN ('AI合成','公检法冒充','刷单返利','投资理财','情感诈骗','贷款代办')),
  summary     text,
  body        text NOT NULL,
  views       bigint NOT NULL DEFAULT 0,
  status      text NOT NULL DEFAULT 'published' CHECK (status IN ('draft','published','archived')),
  updated_by  text REFERENCES users(id),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
```

## 7. scam_rules (全局)

```sql
CREATE TABLE scam_rules (
  id          text PRIMARY KEY,
  category    text NOT NULL,
  keyword     text NOT NULL,
  weight      int  NOT NULL CHECK (weight BETWEEN 0 AND 100),
  enabled     boolean NOT NULL DEFAULT true,
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE(category, keyword)
);
```

## 8. risk_level_state / risk_level_rules

```sql
CREATE TABLE risk_level_state (
  tenant_id     text PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  active_level  int  NOT NULL DEFAULT 3 CHECK (active_level BETWEEN 1 AND 5),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE risk_level_rules (
  id          text PRIMARY KEY,
  tenant_id   text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  level       int  NOT NULL CHECK (level BETWEEN 1 AND 5),
  keyword     text NOT NULL,
  weight      int  NOT NULL CHECK (weight BETWEEN 0 AND 100),
  enabled     boolean NOT NULL DEFAULT true,
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_risk_rules_tenant_level ON risk_level_rules(tenant_id, level);
```

## 9. samples (全局)

```sql
CREATE TABLE samples (
  id              text PRIMARY KEY,
  call_id         text UNIQUE,
  transcript      text,
  duration        text,
  origin          text,
  classification  text,
  status          text NOT NULL DEFAULT '待审核' CHECK (status IN ('待审核','已审核','已驳回')),
  audio_key       text,                    -- MinIO object key
  received_at     timestamptz NOT NULL DEFAULT now()
);
```

## 10. recordings & call_logs

```sql
CREATE TABLE recordings (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  owner_user_id   text REFERENCES users(id),
  phone           text,
  duration        text,
  size_bytes      bigint,
  verdict         text CHECK (verdict IN ('拦截','预警','通过')),
  object_key      text NOT NULL,           -- MinIO 路径
  encryption_key  text,                    -- 客户私有 KMS keyId
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_recordings_tenant ON recordings(tenant_id, created_at DESC);

CREATE TABLE recording_policy (
  tenant_id        text PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  upload_enabled   boolean NOT NULL DEFAULT true,
  updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE call_logs (
  id          text PRIMARY KEY,
  tenant_id   text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id     text REFERENCES users(id),
  phone       text NOT NULL,
  region      text,
  duration    text,
  verdict     text NOT NULL CHECK (verdict IN ('拦截','预警','通过')),
  reason      text,
  risk_score  int  CHECK (risk_score BETWEEN 0 AND 100),
  trace_json      jsonb,                   -- 完整三层引擎结果
  voiceprint_json jsonb,
  script_json     jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_call_logs_tenant_time ON call_logs(tenant_id, created_at DESC);
```

## 11. voice_models / voice_samples (全局)

```sql
CREATE TABLE voice_models (
  id          text PRIMARY KEY,
  version     text NOT NULL UNIQUE,
  accuracy    numeric(5,2) NOT NULL,
  size_bytes  bigint NOT NULL,
  object_key  text NOT NULL,               -- MinIO 路径
  active      boolean NOT NULL DEFAULT false,
  uploaded_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE voice_samples (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  size_bytes  bigint NOT NULL,
  tag         text NOT NULL CHECK (tag IN ('synth','human')),
  object_key  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
```

## 12. agent_config (全局)

```sql
CREATE TABLE agent_config (
  key         text PRIMARY KEY CHECK (key IN ('display_words','whisper','qwen')),
  value       jsonb NOT NULL,
  updated_at  timestamptz NOT NULL DEFAULT now()
);
```

## 13. devices (全局)

```sql
CREATE TABLE devices (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  tenant_id   text REFERENCES tenants(id),  -- 可空：尚未绑定的设备
  type        text NOT NULL CHECK (type IN ('enterprise','family')),
  status      text NOT NULL DEFAULT 'offline' CHECK (status IN ('online','offline','warn')),
  version     text NOT NULL,
  last_seen_at timestamptz,
  contact     text
);
```

## 14. audit_logs (全局)

```sql
CREATE TABLE audit_logs (
  id          bigserial PRIMARY KEY,
  ts          timestamptz NOT NULL DEFAULT now(),
  actor_id    text REFERENCES users(id),
  action      text NOT NULL,
  target      text,
  result      text NOT NULL CHECK (result IN ('成功','失败')),
  ip          inet,
  user_agent  text,
  payload     jsonb
);
CREATE INDEX idx_audit_actor ON audit_logs(actor_id, ts DESC);
CREATE INDEX idx_audit_action ON audit_logs(action, ts DESC);
```

## 15. appeals & admin_applications

```sql
CREATE TABLE appeals (
  id          text PRIMARY KEY,
  user_id     text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tenant_id   text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  type        text NOT NULL CHECK (type IN ('误判申诉','号码举报')),
  number      text NOT NULL,
  reason      text NOT NULL,
  status      text NOT NULL DEFAULT '处理中' CHECK (status IN ('处理中','已通过','已驳回')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  resolved_by text REFERENCES users(id)
);

CREATE TABLE admin_applications (
  id          text PRIMARY KEY,
  user_id     text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  scope       text NOT NULL CHECK (scope IN ('family','biz')),
  reason      text NOT NULL,
  contact     text NOT NULL,
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  reviewed_at timestamptz,
  reviewed_by text REFERENCES users(id)
);
```

## 16. permissions

```sql
CREATE TABLE permissions (
  user_id   text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key       text NOT NULL,            -- e.g. 'pushApp', 'autoBlock', 'viewCalls'
  enabled   boolean NOT NULL DEFAULT false,
  PRIMARY KEY (user_id, key)
);
```

---

## 多租户隔离策略

- 中间件 `tenant.go` 解析 JWT 注入 `ctx.tenantID`。
- 所有 sqlc 查询签名都包含 `tenantID`，无法绕过：

```go
// repo/queries/blacklist.sql
-- name: ListBlacklist :many
SELECT * FROM blacklist
WHERE (tenant_id = $1 OR tenant_id IS NULL)  -- 自己 + 全局
ORDER BY risk DESC LIMIT $2 OFFSET $3;
```

- sysadmin 走专用 query：`WHERE TRUE`。

## 备份与归档

- pg_dump 每日全量 → 对象存储
- `audit_logs` 按月分区（生产环境）
- `recordings.object_key` 指向 MinIO，删除走 `DELETE` 联动清理

## 性能要点

- `blacklist (tenant_id, number)` 索引覆盖热路径。
- `call_logs` 高写入：按 `(tenant_id, created_at)` 索引 + 按月分区。
- 热点黑名单缓存到 Redis：`BL:{tenant}:{number}` → `{risk, category}`，5 分钟 TTL。
