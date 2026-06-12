-- 0002_blacklist.up.sql · 黑/白名单
-- ====================================================================

CREATE TABLE IF NOT EXISTS blacklist (
  id          text PRIMARY KEY,
  tenant_id   text REFERENCES tenants(id) ON DELETE CASCADE,
  number      text NOT NULL,
  reason      text,
  category    text NOT NULL CHECK (category IN ('AI合成','话术诈骗','号码伪冒','其他')),
  risk        int  NOT NULL CHECK (risk BETWEEN 0 AND 100),
  source      text NOT NULL CHECK (source IN ('本地','云端','手动')),
  created_by  text REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_blacklist_tenant_number ON blacklist(tenant_id, number);
CREATE INDEX IF NOT EXISTS idx_blacklist_risk ON blacklist(risk DESC);

CREATE TABLE IF NOT EXISTS whitelist (
  id          text PRIMARY KEY,
  tenant_id   text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  number      text NOT NULL,
  name        text,
  relation    text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, number)
);
