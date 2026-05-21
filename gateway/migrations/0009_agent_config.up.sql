-- 0009_agent_config.up.sql · 智能体配置
-- ====================================================================

CREATE TABLE IF NOT EXISTS agent_config (
  key         text PRIMARY KEY CHECK (key IN ('display_words','whisper','qwen')),
  value       jsonb NOT NULL,
  updated_at  timestamptz NOT NULL DEFAULT now()
);

-- 默认值
INSERT INTO agent_config (key, value) VALUES
('display_words', '["AI 合成可疑","境外信令","公检法冒充","客服伪冒","刷单返利"]'::jsonb),
('whisper', '{"model":"large-v3","language":"zh","vadFilter":true,"beamSize":5,"temperature":0.0}'::jsonb),
('qwen', '{"model":"qwen-max","endpoint":"https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation","temperature":0.2,"topP":0.9,"maxTokens":1024,"systemPrompt":"你是一名反诈通话分析专家。"}'::jsonb)
ON CONFLICT (key) DO NOTHING;
