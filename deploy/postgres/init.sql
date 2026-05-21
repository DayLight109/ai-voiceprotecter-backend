-- 容器启动时由 docker-entrypoint 调用一次
-- 数据库 / 用户已由 POSTGRES_DB / POSTGRES_USER 自动创建，这里只装扩展

CREATE EXTENSION IF NOT EXISTS pgcrypto;          -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pg_trgm;           -- 模糊搜索（号码 / 反诈话术）
