-- 0008_voice_models.up.sql · 声纹模型 + 训练样本
-- ====================================================================

CREATE TABLE IF NOT EXISTS voice_models (
  id          text PRIMARY KEY,
  version     text NOT NULL UNIQUE,
  accuracy    numeric(5,2) NOT NULL,
  size_bytes  bigint NOT NULL,
  object_key  text NOT NULL,
  active      boolean NOT NULL DEFAULT false,
  uploaded_at timestamptz NOT NULL DEFAULT now()
);

-- 同一时间只能有一个 active = true
CREATE UNIQUE INDEX IF NOT EXISTS uniq_voice_models_active ON voice_models((1)) WHERE active = true;

CREATE TABLE IF NOT EXISTS voice_samples (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  size_bytes  bigint NOT NULL,
  tag         text NOT NULL CHECK (tag IN ('synth','human')),
  object_key  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
