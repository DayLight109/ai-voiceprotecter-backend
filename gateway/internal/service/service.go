// Package service 业务逻辑层（保留接入点，当前已部分被绕过）。
//
// 现状（2026-05）：
//   骨架阶段 handlers 已直连 repo 完成 ~80% 的 CRUD（auth / blacklist /
//   knowledge / 等），service 层未实例化。这是有意的权衡 —— 当业务规则
//   足够简单（CRUD + 角色限制 + UUID 生成）时多加一层只是仪式感。
//
// 何时应该回填这一层？
//   1. 同一业务规则被两个 handler 共享（DRY 阈值）
//   2. 需要跨 repo 表的事务（用 repo.WithTx 封装）
//   3. 引入异步流程（发邮件、加密 PII、调外部接口）
//   4. 业务规则越来越复杂，handler 函数 > 80 行
//
// 命名占位（按模块）：
//   auth.go         登录 / 注册 / refresh / 撤销
//   identity.go     证件认证
//   blacklist.go    黑名单 CRUD + 缓存
//   whitelist.go
//   knowledge.go
//   rules.go
//   risk_level.go
//   samples.go      含自动分析 → 派生规则 / 知识
//   recordings.go   预签名 / 删除 / 策略
//   calls.go
//   users.go
//   appeals.go
//   admin_apply.go
//   permissions.go
//   devices.go
//   audit.go
//   voice_models.go
//   voice_samples.go
//   agents.go
//   dashboard.go
package service
