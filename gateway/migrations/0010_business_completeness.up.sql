-- 0010_business_completeness.up.sql
-- 业务表完善：软删除 / updated_at / 唯一约束 / 缺失外键 / 审核字段 / 复合索引
--
-- 适用范围：业务实体表加生命周期字段；纯日志表(call_logs / audit_logs / sessions)不加。
-- 全部用 IF NOT EXISTS / IF EXISTS 兼容多次执行。

-- ───────────────────────────────────────────────────────────────────────
-- 1. 软删除 deleted_at —— 用户可"删除"的业务实体
-- ───────────────────────────────────────────────────────────────────────
ALTER TABLE users               ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE blacklist           ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE whitelist           ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE knowledge_articles  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE samples             ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE recordings          ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE devices             ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE appeals             ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE admin_applications  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE risk_level_rules    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE voice_models        ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE voice_samples       ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE scam_rules          ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- ───────────────────────────────────────────────────────────────────────
-- 2. updated_at —— 缺失的写操作时间戳
-- ───────────────────────────────────────────────────────────────────────
ALTER TABLE users              ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE blacklist          ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE whitelist          ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE samples            ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE recordings         ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE devices            ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE appeals            ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE admin_applications ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE voice_models       ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE voice_samples      ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE knowledge_articles ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE scam_rules         ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE risk_level_rules   ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- ───────────────────────────────────────────────────────────────────────
-- 3. samples 表加 tenant_id（多租户隔离漏洞）+ 审核字段 + call_id 外键
-- ───────────────────────────────────────────────────────────────────────
ALTER TABLE samples ADD COLUMN IF NOT EXISTS tenant_id TEXT REFERENCES tenants(id) ON DELETE CASCADE;

-- 回填：用 call_logs.tenant_id 关联回填，未关联的兜底到 global 租户
UPDATE samples s
   SET tenant_id = c.tenant_id
  FROM call_logs c
 WHERE s.call_id = c.id AND s.tenant_id IS NULL;

UPDATE samples
   SET tenant_id = (SELECT id FROM tenants WHERE kind = 'global' LIMIT 1)
 WHERE tenant_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_samples_tenant ON samples(tenant_id);

ALTER TABLE samples
  ADD COLUMN IF NOT EXISTS reviewed_by         TEXT REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS reviewed_at         TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS linked_blacklist_id TEXT REFERENCES blacklist(id) ON DELETE SET NULL;

-- samples.call_id 关联 call_logs.id（之前未建外键）
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fk_samples_call'
  ) THEN
    ALTER TABLE samples
      ADD CONSTRAINT fk_samples_call FOREIGN KEY (call_id) REFERENCES call_logs(id) ON DELETE SET NULL;
  END IF;
END $$;

-- ───────────────────────────────────────────────────────────────────────
-- 4. 唯一约束（防重）
-- ───────────────────────────────────────────────────────────────────────
-- blacklist (tenant_id, number) 唯一：null tenant_id（全局黑名单）也允许重复，故用 COALESCE
CREATE UNIQUE INDEX IF NOT EXISTS uniq_blacklist_tenant_number
  ON blacklist(COALESCE(tenant_id, ''), number) WHERE deleted_at IS NULL;

-- users.email 唯一：占位"—"和 NULL 不参与唯一性
CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_email
  ON users(email) WHERE email IS NOT NULL AND email <> '' AND email <> '—' AND deleted_at IS NULL;

-- permissions (user_id, key) 唯一
CREATE UNIQUE INDEX IF NOT EXISTS uniq_permissions_user_key
  ON permissions(user_id, key);

-- ───────────────────────────────────────────────────────────────────────
-- 5. 审核 / 审计字段（业务可追溯）
-- ───────────────────────────────────────────────────────────────────────
ALTER TABLE appeals             ADD COLUMN IF NOT EXISTS decision_note TEXT;
ALTER TABLE users               ADD COLUMN IF NOT EXISTS updated_by    TEXT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE voice_models        ADD COLUMN IF NOT EXISTS description   TEXT;
ALTER TABLE voice_models        ADD COLUMN IF NOT EXISTS created_by    TEXT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE risk_level_rules    ADD COLUMN IF NOT EXISTS created_by    TEXT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE risk_level_rules    ADD COLUMN IF NOT EXISTS version       INT  NOT NULL DEFAULT 1;
ALTER TABLE users               ADD COLUMN IF NOT EXISTS failed_login_count INT NOT NULL DEFAULT 0;
ALTER TABLE users               ADD COLUMN IF NOT EXISTS locked_until  TIMESTAMPTZ;

-- ───────────────────────────────────────────────────────────────────────
-- 6. 复合索引（依据 repo 层 ORDER BY 推导）
-- ───────────────────────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_admin_apps_status_time   ON admin_applications(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_admin_apps_user_time     ON admin_applications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_appeals_tenant_status    ON appeals(tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_appeals_user_time        ON appeals(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_samples_status_time      ON samples(status, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_pub_cat_time   ON knowledge_articles(category, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_devices_type_tenant_seen ON devices(type, tenant_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_devices_last_seen        ON devices(last_seen_at);
CREATE INDEX IF NOT EXISTS idx_users_tenant_time        ON users(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_whitelist_tenant_time    ON whitelist(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_voice_models_time        ON voice_models(uploaded_at DESC);
CREATE INDEX IF NOT EXISTS idx_voice_samples_time       ON voice_samples(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_blacklist_category       ON blacklist(category);
CREATE INDEX IF NOT EXISTS idx_call_logs_verdict_time   ON call_logs(verdict, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_recordings_owner_time    ON recordings(owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_expires         ON sessions(expires_at);

-- ───────────────────────────────────────────────────────────────────────
-- 7. ON DELETE 行为修正：删除用户 / 租户时业务实体保留（SET NULL）
-- 用 DO 块兼容已有/未有约束的情况
-- ───────────────────────────────────────────────────────────────────────
DO $$
DECLARE
  fk_fix RECORD;
BEGIN
  FOR fk_fix IN
    SELECT * FROM (VALUES
      ('blacklist',          'blacklist_created_by_fkey',          'created_by',     'users',  'id'),
      ('knowledge_articles', 'knowledge_articles_updated_by_fkey', 'updated_by',     'users',  'id'),
      ('call_logs',          'call_logs_user_id_fkey',             'user_id',        'users',  'id'),
      ('recordings',         'recordings_owner_user_id_fkey',      'owner_user_id',  'users',  'id'),
      ('devices',            'devices_tenant_id_fkey',             'tenant_id',      'tenants','id'),
      ('appeals',            'appeals_resolved_by_fkey',           'resolved_by',    'users',  'id'),
      ('admin_applications', 'admin_applications_reviewed_by_fkey','reviewed_by',    'users',  'id'),
      ('audit_logs',         'audit_logs_actor_id_fkey',           'actor_id',       'users',  'id')
    ) AS t(table_name, fk_name, col, ref_table, ref_col)
  LOOP
    -- 先 drop 再 add；若原本无此约束，drop 用 IF EXISTS 不会报错
    EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', fk_fix.table_name, fk_fix.fk_name);
    EXECUTE format(
      'ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %I(%I) ON DELETE SET NULL',
      fk_fix.table_name, fk_fix.fk_name, fk_fix.col, fk_fix.ref_table, fk_fix.ref_col
    );
  END LOOP;
END $$;
