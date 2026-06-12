-- 0014_drop_unused_soft_delete.up.sql
-- 0010 为 13 张表加的 deleted_at 软删列，代码（repo 层全部查询/删除）从未使用：
-- 查询不过滤 deleted_at，删除全部是硬 DELETE。空挂的列引人误以为有软删语义，
-- 还会让唯一索引在未来真做软删时直接冲突（索引均非 partial）。先移除；
-- 将来要软删时连同 partial unique index 一起重新设计。
-- （0010 同期加的 failed_login_count / locked_until 保留 —— 属登录防爆破功能，待安全专项实现。）

ALTER TABLE users              DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE blacklist          DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE whitelist          DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE knowledge_articles DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE samples            DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE recordings         DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE devices            DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE appeals            DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE admin_applications DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE risk_level_rules   DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE voice_models       DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE voice_samples      DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE scam_rules         DROP COLUMN IF EXISTS deleted_at;
