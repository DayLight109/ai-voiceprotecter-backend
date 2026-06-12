-- 0006_devices_audit.up.sql · 设备 + 审计日志
-- ====================================================================

CREATE TABLE IF NOT EXISTS devices (
  id           text PRIMARY KEY,
  name         text NOT NULL,
  tenant_id    text REFERENCES tenants(id),
  type         text NOT NULL CHECK (type IN ('enterprise','family')),
  status       text NOT NULL DEFAULT 'offline' CHECK (status IN ('online','offline','warn')),
  version      text NOT NULL,
  last_seen_at timestamptz,
  contact      text
);

CREATE TABLE IF NOT EXISTS audit_logs (
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
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action, ts DESC);
