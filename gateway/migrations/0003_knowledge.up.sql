-- 0003_knowledge.up.sql · 知识库
-- ====================================================================

CREATE TABLE IF NOT EXISTS knowledge_articles (
  id          text PRIMARY KEY,
  title       text NOT NULL,
  category    text NOT NULL CHECK (category IN ('AI合成','公检法冒充','刷单返利','投资理财','情感诈骗','贷款代办')),
  summary     text,
  body        text NOT NULL,
  views       bigint NOT NULL DEFAULT 0,
  status      text NOT NULL DEFAULT 'published' CHECK (status IN ('draft','published','archived')),
  updated_by  text REFERENCES users(id),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_knowledge_category ON knowledge_articles(category);
