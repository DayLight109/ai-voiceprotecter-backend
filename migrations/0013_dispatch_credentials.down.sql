-- 0013_dispatch_credentials.down.sql
-- ====================================================================

DROP INDEX IF EXISTS idx_blacklist_dispatched;

-- 回滚 source CHECK 前先把 '举报' 来源改写为 '手动'，避免约束加不回去
UPDATE blacklist SET source = '手动' WHERE source = '举报';
ALTER TABLE blacklist DROP CONSTRAINT IF EXISTS blacklist_source_check;
ALTER TABLE blacklist ADD CONSTRAINT blacklist_source_check
  CHECK (source IN ('本地','云端','手动'));

ALTER TABLE blacklist DROP COLUMN IF EXISTS dispatched;

DROP TABLE IF EXISTS identity_photos;
ALTER TABLE identity_credentials DROP COLUMN IF EXISTS masked;
ALTER TABLE identity_credentials DROP COLUMN IF EXISTS updated_at;

-- agent_config 的嵌套修复为单向数据清洗，不做回滚
