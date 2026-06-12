-- 0012_me_profile.up.sql · 个人中心补全：头像 / 会话最近活跃
-- ====================================================================
-- 支撑 /settings 页：头像上传、登录设备列表。
-- 头像存 Postgres（bytea）而非 MinIO —— 保证本地无对象存储时功能仍可用。

-- 用户头像：与 users 1:1，删除用户级联清除
CREATE TABLE IF NOT EXISTS user_avatars (
  user_id      text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  content_type text NOT NULL,
  bytes        bytea NOT NULL,
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- 会话最近活跃时间：登录设备列表展示用；回填为创建时间
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_seen_at timestamptz;
UPDATE sessions SET last_seen_at = created_at WHERE last_seen_at IS NULL;
