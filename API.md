# API 清单

> Base URL: `http://localhost:8080/api/v1`
> 鉴权：`Authorization: Bearer <jwt>`（除 `/auth/login`、`/auth/register`、`/health`）
> 时间：RFC3339 UTC

## 响应包装

```jsonc
// 成功
{ "data": <payload>, "meta": { "page": 1, "pageSize": 20, "total": 42 } }

// 错误
{ "error": { "code": "BLACKLIST_NUMBER_EXISTS", "message": "号码已存在", "field": "number" } }
```

---

## 1. 认证 · `/auth`

| Method | Path | 说明 | 角色 |
|---|---|---|---|
| POST | `/auth/login` | 账号 + 密码登录，返 access + refresh | 公开 |
| POST | `/auth/register` | 新用户注册（家庭/企业） | 公开 |
| POST | `/auth/refresh` | refresh → 新 access | 公开 |
| POST | `/auth/logout` | 注销当前 refresh | 任意 |
| POST | `/auth/verify-id` | 上传身份证 OCR + 公安核验 | 公开 |
| POST | `/auth/verify-liveness` | 活体检测视频核验 | 公开 |
| POST | `/auth/verify-fingerprint` | 指纹模板上传（本地加密） | 公开 |
| GET | `/auth/me` | 当前用户信息 | 任意 |

请求样例：

```jsonc
// POST /auth/login
{ "account": "138 0013 4921", "password": "demo" }
// → 200
{ "data": {
    "accessToken": "eyJ...",
    "refreshToken": "rt-...",
    "user": { "id": "u1", "name": "王磊", "role": "family", "tenantId": "t1" }
  }
}
```

## 2. 身份认证 · `/me/credentials`

| Method | Path | 说明 |
|---|---|---|
| GET | `/me/credentials` | 我的全部证件状态（5 种） |
| POST | `/me/credentials/{kind}` | 提交某证件认证 (`phone\|id_card\|passport\|military\|hk_mo`) |
| GET | `/me/identity-modes` | 线下/亲属/关怀 三开关 |
| PATCH | `/me/identity-modes` | 调整开关 |

## 3. 分析引擎 · `/analyze`

| Method | Path | 说明 |
|---|---|---|
| POST | `/analyze` | 三层引擎并发判决（内部转发至 AI 服务） |

```jsonc
// POST /analyze
{
  "callId": "t-001",
  "shownNumber": "+86-138-0013-4921",
  "signalOriginCC": "MM",
  "audioSeconds": 6.4,
  "transcriptHint": "打到这个安全账户"
}
// → 200
{ "data": {
    "callId": "t-001",
    "trace":      { "shownRegistry": "CN/BJ", "actualOrigin": "MM", "mismatch": true, "risk": 86 },
    "voiceprint": { "synthProbability": 0.94, "verdict": "SYNTH", "risk": 92 },
    "script":     { "hits": [{"category":"引导转账","weight":92}], "risk": 96 },
    "riskScore":  94,
    "riskLevel":  "BLOCK",
    "action":     "block",
    "latencyMillis": 102
  }
}
```

## 4. 黑/白名单 · `/blacklist` `/whitelist`

> 三处前端复用，靠 `tenant_id` 区分。sysadmin 可写 `tenant_id IS NULL` 的全局名单。

| Method | Path | 说明 | 角色 |
|---|---|---|---|
| GET | `/blacklist?q=&category=&page=&pageSize=&sortBy=risk` | 列表 | 任意 |
| POST | `/blacklist` | 新增 | family/biz/admin 等 |
| PUT | `/blacklist/{id}` | 更新 | 同上 |
| DELETE | `/blacklist/{id}` | 删除 | 同上 |
| POST | `/blacklist/import` | CSV/XLSX 导入 (multipart) | family-admin/admin/sysadmin |
| GET | `/blacklist/export` | CSV 导出 | 同上 |
| GET | `/whitelist` ... | 与黑名单对称 | |

## 5. 知识库 · `/knowledge`

| Method | Path | 说明 | 角色 |
|---|---|---|---|
| GET | `/knowledge?category=&q=` | 列表 | 任意 |
| GET | `/knowledge/{id}` | 详情（自动 view++） | 任意 |
| POST | `/knowledge` | 发布 | sysadmin |
| PUT | `/knowledge/{id}` | 更新 | sysadmin |
| DELETE | `/knowledge/{id}` | 删除 | sysadmin |

## 6. 规则库 · `/scam-rules`

| Method | Path | 说明 | 角色 |
|---|---|---|---|
| GET | `/scam-rules?category=` | 列表 | 任意 |
| POST/PUT/DELETE | `/scam-rules[/{id}]` | CRUD | sysadmin |

## 7. 风控等级 · `/risk-level`

| Method | Path | 说明 |
|---|---|---|
| GET | `/risk-level/state` | 当前激活等级 (1-5) |
| PUT | `/risk-level/state` | 切换激活 `{"level": 3}` |
| GET | `/risk-level/rules?level=` | 自定义规则列表 |
| POST/PUT/DELETE | `/risk-level/rules[/{id}]` | 增删改 |

## 8. 样本审核 · `/samples`

| Method | Path | 说明 | 角色 |
|---|---|---|---|
| GET | `/samples?status=` | 列表 | sysadmin |
| GET | `/samples/{id}` | 详情 | sysadmin |
| POST | `/samples/{id}/analyze` | 自动学习 → 更新规则库 + 知识库 | sysadmin |
| POST | `/samples/{id}/reject` | 驳回 | sysadmin |
| GET | `/samples/{id}/export-doc` | 返回 `application/msword` | sysadmin |

## 9. 音频分析配置 · `/voice-models` `/voice-samples`

| Method | Path | 说明 |
|---|---|---|
| GET | `/voice-models` | 模型版本列表 + 准确率 |
| POST | `/voice-models` | 直传 ONNX (multipart) |
| POST | `/voice-models/{id}/activate` | 切换激活 |
| DELETE | `/voice-models/{id}` | 删除 |
| GET/POST/DELETE | `/voice-samples` | 声纹样本（synth / human） |

## 10. 智能体 · `/agents`

| Method | Path | 说明 |
|---|---|---|
| GET / PUT | `/agents/display-words` | 端侧显示词数组（≤12） |
| GET / PUT | `/agents/whisper` | model / language / vad / beamSize / temp |
| GET / PUT | `/agents/qwen` | model / endpoint / apiKey / temp / topP / maxTokens / systemPrompt |

## 11. 录音管理 · `/recordings`

| Method | Path | 说明 |
|---|---|---|
| GET | `/recordings?ownerId=&verdict=` | 列表 |
| GET | `/recordings/{id}/download` | 返回预签名 URL（有效期 5 分钟） |
| DELETE | `/recordings/{id}` | 删除（MinIO 同步擦） |
| GET / PUT | `/recordings/policy` | `{"uploadEnabled": true}` |

## 12. 通话记录 · `/calls`

| Method | Path | 说明 |
|---|---|---|
| GET | `/calls?phone=&verdict=&from=&to=` | 列表 |
| GET | `/calls/{id}` | 详情 + 三层引擎拆解 |

## 13. 用户管理 · `/users`

| Method | Path | 说明 | 角色 |
|---|---|---|---|
| GET | `/users` | 列表 | admin / family-admin |
| POST | `/users` | 新增成员 | admin / family-admin |
| PUT | `/users/{id}` | 编辑 | admin / family-admin |
| DELETE | `/users/{id}` | 删除 | admin / family-admin |

## 14. 申诉 · `/appeals`

| Method | Path | 说明 |
|---|---|---|
| GET | `/appeals` | 当前用户的历史 |
| POST | `/appeals` | 误判申诉 / 号码举报 |
| PUT | `/appeals/{id}/status` | 通过 / 驳回（sysadmin） |

## 15. 管理员申请 · `/admin-apply`

| Method | Path | 说明 |
|---|---|---|
| POST | `/admin-apply` | 提交申请 |
| GET | `/admin-apply/status` | 我的状态 |
| GET | `/admin-apply` | 待审列表（sysadmin） |
| PUT | `/admin-apply/{id}/review` | 通过 / 驳回（sysadmin） |

## 16. 权限设置 · `/permissions`

| Method | Path | 说明 |
|---|---|---|
| GET / PUT | `/permissions/family` | 家庭权限位图 |
| GET / PUT | `/permissions/biz` | 企业权限位图 |

## 17. 设备 · `/devices`

| Method | Path | 说明 |
|---|---|---|
| GET | `/devices?type=enterprise\|family` | 列表 |
| POST/PUT/DELETE | `/devices[/{id}]` | CRUD |
| GET | `/devices/audit?actor=` | 审计日志（DeviceManager 行为日志 tab） |

## 18. 审计 · `/audit`

| Method | Path | 说明 |
|---|---|---|
| GET | `/audit?actor=&action=&from=&to=` | 审计日志 | sysadmin |

## 19. 风险大屏 · `/dashboard`

| Method | Path | 说明 |
|---|---|---|
| GET | `/dashboard/risk-index` | 风险指数 + DEFCON |
| GET | `/dashboard/regions` | 高危地区分布 |
| GET | `/dashboard/events?limit=` | 告警事件流 |

## 20. Warroom 兼容 · 旧协议

| Method | Path | 说明 |
|---|---|---|
| GET | `/health` | 健康检查 |
| GET | `/stats` | 全局计数器 |
| GET | `/defcon` / POST `/defcon` | DEFCON 等级 |
| GET | `/feed?n=32` | 最近 N 条事件 |
| GET | `/feed/stream` | **SSE**：`event: feed\ndata: {...}` |
| GET | `/threats` | 最近 16 条 danger 级事件 |

---

## 错误码字典（节选）

| code | HTTP | 含义 |
|---|---|---|
| `AUTH_INVALID_CREDENTIALS` | 401 | 账号或密码错误 |
| `AUTH_TOKEN_EXPIRED` | 401 | access 过期，请走 /refresh |
| `AUTH_REFRESH_INVALID` | 401 | refresh 被吊销或过期 |
| `RBAC_FORBIDDEN` | 403 | 当前角色无权访问 |
| `TENANT_MISMATCH` | 403 | 越权访问他租户数据 |
| `RESOURCE_NOT_FOUND` | 404 | 主键不存在 |
| `VALIDATION_FAILED` | 422 | 字段校验失败，看 `field` |
| `RATE_LIMITED` | 429 | 触发限流 |
| `BLACKLIST_NUMBER_EXISTS` | 409 | 号码已在黑名单 |
| `NOT_IMPLEMENTED` | 501 | 端点尚未接入业务层（骨架阶段） |
| `AI_UPSTREAM_ERROR` | 502 | gateway 调 AI 子服务失败 |
| `AI_UPSTREAM_TIMEOUT` | 504 | AI 子服务超时 |
| `MODEL_UNAVAILABLE` | 503 | AI 服务的 Whisper / ONNX 模型未加载 |
| `LLM_UNAVAILABLE` | 503 | AI 服务的千问 API key 未配置 |
| `INTERNAL_ERROR` | 500 | 兜底错误 |

## 骨架阶段（P0）的响应约定

| 端点类型 | 当前响应 |
|---|---|
| `GET /health`、`/healthz` | 200 ok |
| `GET <资源>` 列表 | 200 + `{"data":[], "meta":{"page":1,"pageSize":20,"total":0}}` |
| `GET /<资源>/{id}` 详情 | 501 NOT_IMPLEMENTED |
| `POST/PUT/DELETE/PATCH` | 501 NOT_IMPLEMENTED |
| `POST /analyze` | 502 AI_UPSTREAM_ERROR（AI 未就绪）或转发 AI 503 |
| `GET /feed/stream` | 200 SSE，hello + 心跳；真实事件由 analyze 写入后才出现 |

P1 起按模块逐个把 501 换成真实实现。
