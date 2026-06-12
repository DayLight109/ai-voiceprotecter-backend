-- 0013_dispatch_credentials.up.sql · 黑名单下发工作流 / 证件照片 / 智能体配置数据修复
-- ====================================================================

-- 1. 黑名单"待下发"工作流：举报通过自动入库的条目 dispatched=false，
--    管理员在黑名单页手动下发后才生效。存量条目一律视为已生效。
ALTER TABLE blacklist ADD COLUMN IF NOT EXISTS dispatched boolean NOT NULL DEFAULT true;

-- source 允许 '举报'（号码举报通过后自动入库的来源标记）
ALTER TABLE blacklist DROP CONSTRAINT IF EXISTS blacklist_source_check;
ALTER TABLE blacklist ADD CONSTRAINT blacklist_source_check
  CHECK (source IN ('本地','云端','手动','举报'));

CREATE INDEX IF NOT EXISTS idx_blacklist_dispatched ON blacklist(dispatched) WHERE dispatched = false;

-- 2. 证件照片：multipart 上传，bytea 入库（与 user_avatars 同策略，
--    保证本地无对象存储时功能可用）。删除用户级联清除。
CREATE TABLE IF NOT EXISTS identity_photos (
  user_id      text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('phone','id_card','passport','military','hk_mo')),
  slot         text NOT NULL CHECK (slot IN ('face','emblem','main')),
  name         text NOT NULL DEFAULT '',
  content_type text NOT NULL,
  bytes        bytea NOT NULL,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, kind, slot)
);

-- 证件记录补充脱敏展示值与更新时间（identity 页"已提交记录"卡片展示用）
ALTER TABLE identity_credentials ADD COLUMN IF NOT EXISTS masked text;
ALTER TABLE identity_credentials ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

-- 3. 修复智能体配置历史脏数据：旧版 putAgent 把整个请求体 {"value": X} 存进
--    value 列导致双重嵌套，这里剥掉一层。仅处理"单键且键名为 value 的对象"。
UPDATE agent_config
   SET value = value->'value'
 WHERE jsonb_typeof(value) = 'object'
   AND value ? 'value'
   AND (SELECT count(*) FROM jsonb_object_keys(value)) = 1;
