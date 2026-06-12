-- 0001_init.up.sql · 初始化：租户 / 用户 / 会话 / 证件
-- ====================================================================

CREATE TABLE IF NOT EXISTS tenants (
  id          text PRIMARY KEY,
  kind        text NOT NULL CHECK (kind IN ('family','enterprise','global')),
  name        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name            text NOT NULL,
  phone           text UNIQUE,
  id_card_hash    text,
  email           text,
  password_hash   text NOT NULL,
  role            text NOT NULL CHECK (role IN ('family','biz','family_admin','admin','sysadmin')),
  status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active','review','suspended')),
  dept            text,
  last_login_at   timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_users_tenant ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);

CREATE TABLE IF NOT EXISTS identity_credentials (
  id           text PRIMARY KEY,
  user_id      text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('phone','id_card','passport','military','hk_mo')),
  value_hash   text,
  verified     boolean NOT NULL DEFAULT false,
  verified_at  timestamptz,
  UNIQUE(user_id, kind)
);

CREATE TABLE IF NOT EXISTS sessions (
  jti                  text PRIMARY KEY,
  user_id              text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_token_hash   text NOT NULL,
  expires_at           timestamptz NOT NULL,
  revoked              boolean NOT NULL DEFAULT false,
  user_agent           text,
  ip                   inet,
  created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
