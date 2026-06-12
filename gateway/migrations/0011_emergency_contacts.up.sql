-- 0010_emergency_contacts.up.sql · 用户级紧急联系人
-- ====================================================================
-- 家庭用户在 /settings 个人信息卡里维护，发生拦截时一并推送。
-- 与 users 是 N:1，user_id 维度做隔离；同一用户内号码唯一。

CREATE TABLE IF NOT EXISTS emergency_contacts (
  id          text PRIMARY KEY,
  user_id     text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        text NOT NULL,
  phone       text NOT NULL,
  relation    text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, phone)
);

CREATE INDEX IF NOT EXISTS idx_emergency_contacts_user
  ON emergency_contacts (user_id, created_at DESC);
