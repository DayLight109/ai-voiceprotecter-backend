-- 0010_business_completeness.down.sql

-- 索引（CREATE INDEX IF NOT EXISTS 的反向）
DROP INDEX IF EXISTS uniq_blacklist_tenant_number;
DROP INDEX IF EXISTS uniq_users_email;
DROP INDEX IF EXISTS uniq_permissions_user_key;
DROP INDEX IF EXISTS idx_samples_tenant;
DROP INDEX IF EXISTS idx_admin_apps_status_time;
DROP INDEX IF EXISTS idx_admin_apps_user_time;
DROP INDEX IF EXISTS idx_appeals_tenant_status;
DROP INDEX IF EXISTS idx_appeals_user_time;
DROP INDEX IF EXISTS idx_samples_status_time;
DROP INDEX IF EXISTS idx_knowledge_pub_cat_time;
DROP INDEX IF EXISTS idx_devices_type_tenant_seen;
DROP INDEX IF EXISTS idx_devices_last_seen;
DROP INDEX IF EXISTS idx_users_tenant_time;
DROP INDEX IF EXISTS idx_whitelist_tenant_time;
DROP INDEX IF EXISTS idx_voice_models_time;
DROP INDEX IF EXISTS idx_voice_samples_time;
DROP INDEX IF EXISTS idx_blacklist_category;
DROP INDEX IF EXISTS idx_call_logs_verdict_time;
DROP INDEX IF EXISTS idx_recordings_owner_time;
DROP INDEX IF EXISTS idx_sessions_expires;

-- samples 关联回退
ALTER TABLE samples DROP CONSTRAINT IF EXISTS fk_samples_call;
ALTER TABLE samples DROP COLUMN IF EXISTS reviewed_by;
ALTER TABLE samples DROP COLUMN IF EXISTS reviewed_at;
ALTER TABLE samples DROP COLUMN IF EXISTS linked_blacklist_id;
ALTER TABLE samples DROP COLUMN IF EXISTS tenant_id;

-- 审核 / 审计字段
ALTER TABLE appeals          DROP COLUMN IF EXISTS decision_note;
ALTER TABLE users            DROP COLUMN IF EXISTS updated_by;
ALTER TABLE users            DROP COLUMN IF EXISTS failed_login_count;
ALTER TABLE users            DROP COLUMN IF EXISTS locked_until;
ALTER TABLE voice_models     DROP COLUMN IF EXISTS description;
ALTER TABLE voice_models     DROP COLUMN IF EXISTS created_by;
ALTER TABLE risk_level_rules DROP COLUMN IF EXISTS created_by;
ALTER TABLE risk_level_rules DROP COLUMN IF EXISTS version;

-- updated_at
ALTER TABLE users              DROP COLUMN IF EXISTS updated_at;
ALTER TABLE blacklist          DROP COLUMN IF EXISTS updated_at;
ALTER TABLE whitelist          DROP COLUMN IF EXISTS updated_at;
ALTER TABLE samples            DROP COLUMN IF EXISTS updated_at;
ALTER TABLE recordings         DROP COLUMN IF EXISTS updated_at;
ALTER TABLE devices            DROP COLUMN IF EXISTS updated_at;
ALTER TABLE appeals            DROP COLUMN IF EXISTS updated_at;
ALTER TABLE admin_applications DROP COLUMN IF EXISTS updated_at;
ALTER TABLE voice_models       DROP COLUMN IF EXISTS updated_at;
ALTER TABLE voice_samples      DROP COLUMN IF EXISTS updated_at;
ALTER TABLE knowledge_articles DROP COLUMN IF EXISTS created_at;
ALTER TABLE scam_rules         DROP COLUMN IF EXISTS updated_at;
ALTER TABLE risk_level_rules   DROP COLUMN IF EXISTS updated_at;

-- deleted_at
ALTER TABLE users               DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE blacklist           DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE whitelist           DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE knowledge_articles  DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE samples             DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE recordings          DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE devices             DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE appeals             DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE admin_applications  DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE risk_level_rules    DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE voice_models        DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE voice_samples       DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE scam_rules          DROP COLUMN IF EXISTS deleted_at;

-- ON DELETE 行为不还原（默认 NO ACTION 即可，不需要 DROP）
