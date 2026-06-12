-- 0004_rules_risk.up.sql · 诈骗规则 + 风控等级
-- ====================================================================

CREATE TABLE IF NOT EXISTS scam_rules (
  id          text PRIMARY KEY,
  category    text NOT NULL,
  keyword     text NOT NULL,
  weight      int  NOT NULL CHECK (weight BETWEEN 0 AND 100),
  enabled     boolean NOT NULL DEFAULT true,
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE(category, keyword)
);

CREATE TABLE IF NOT EXISTS risk_level_state (
  tenant_id     text PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  active_level  int  NOT NULL DEFAULT 3 CHECK (active_level BETWEEN 1 AND 5),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS risk_level_rules (
  id          text PRIMARY KEY,
  tenant_id   text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  level       int  NOT NULL CHECK (level BETWEEN 1 AND 5),
  keyword     text NOT NULL,
  weight      int  NOT NULL CHECK (weight BETWEEN 0 AND 100),
  enabled     boolean NOT NULL DEFAULT true,
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_risk_rules_tenant_level ON risk_level_rules(tenant_id, level);
