-- seed.sql · 系统初始数据
--
-- 不含任何演示业务数据。仅创建：
--   1. 一个 global 租户（系统级表的归属）
--   2. 一个 sysadmin 账号（首次登录用，请立刻改密）
--
-- 启动后所有家庭 / 企业租户、用户、黑/白名单、知识、规则、设备
-- 全部由各自的 CRUD 接口由 sysadmin/admin/家庭管理员 创建。
-- ====================================================================

BEGIN;

-- 默认全局租户（sysadmin 归属、全局黑名单 / 知识 / 规则的"虚拟"挂载点）
INSERT INTO tenants (id, kind, name) VALUES
  ('t-global', 'global', '全局')
ON CONFLICT (id) DO NOTHING;

-- 系统管理员
-- 默认密码：ChangeMe!2026  （bcrypt cost=12 hash 见下；强烈建议首次登录立即修改）
-- 你也可以删掉这行，改用 CLI 工具创建首个 sysadmin。
INSERT INTO users (id, tenant_id, name, phone, email, password_hash, role, status) VALUES
  ('u-sysadmin-bootstrap',
   't-global',
   'sysadmin',
   NULL,
   'sysadmin@sentinel.local',
   '$2a$12$lP3kj2N0Y0qC7sQwR6S1n.E5gM7Q6sB8a1KqOq7p1lqYqQ3kIyB8e',  -- ChangeMe!2026
   'sysadmin',
   'active')
ON CONFLICT (id) DO NOTHING;

COMMIT;
