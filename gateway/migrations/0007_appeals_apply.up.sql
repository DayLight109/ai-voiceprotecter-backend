-- 0007_appeals_apply.up.sql · 申诉 + 管理员申请 + 权限
-- ====================================================================

CREATE TABLE IF NOT EXISTS appeals (
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

CREATE TABLE IF NOT EXISTS admin_applications (
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

CREATE TABLE IF NOT EXISTS permissions (
  user_id   text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key       text NOT NULL,
  enabled   boolean NOT NULL DEFAULT false,
  PRIMARY KEY (user_id, key)
);
