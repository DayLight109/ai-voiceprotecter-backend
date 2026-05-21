-- 0005_samples_recordings.up.sql · 样本审核 / 录音 / 通话日志
-- ====================================================================

CREATE TABLE IF NOT EXISTS samples (
  id              text PRIMARY KEY,
  call_id         text UNIQUE,
  transcript      text,
  duration        text,
  origin          text,
  classification  text,
  status          text NOT NULL DEFAULT '待审核' CHECK (status IN ('待审核','已审核','已驳回')),
  audio_key       text,
  received_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS recordings (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  owner_user_id   text REFERENCES users(id),
  phone           text,
  duration        text,
  size_bytes      bigint,
  verdict         text CHECK (verdict IN ('拦截','预警','通过')),
  object_key      text NOT NULL,
  encryption_key  text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_recordings_tenant_time ON recordings(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS recording_policy (
  tenant_id        text PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  upload_enabled   boolean NOT NULL DEFAULT true,
  updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS call_logs (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id         text REFERENCES users(id),
  phone           text NOT NULL,
  region          text,
  duration        text,
  verdict         text NOT NULL CHECK (verdict IN ('拦截','预警','通过')),
  reason          text,
  risk_score      int  CHECK (risk_score BETWEEN 0 AND 100),
  trace_json      jsonb,
  voiceprint_json jsonb,
  script_json     jsonb,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_call_logs_tenant_time ON call_logs(tenant_id, created_at DESC);
