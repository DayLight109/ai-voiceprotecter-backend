.PHONY: help dev up down logs migrate migrate-down seed gateway-build gateway-run ai-run ai-install fmt lint test clean

SHELL := /bin/bash
COMPOSE := docker compose -f deploy/docker-compose.yml --env-file .env

help: ## 列出全部 target
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: up migrate seed ## 一键起整套 + 迁移 + 种子

up: ## 启动 docker-compose
	$(COMPOSE) up -d

down: ## 停止
	$(COMPOSE) down

down-v: ## 停止并清卷
	$(COMPOSE) down -v

logs: ## 跟随日志
	$(COMPOSE) logs -f --tail=200

ps: ## 查看容器状态
	$(COMPOSE) ps

# ── Migrations (golang-migrate) ─────────────────────────────────────────
migrate: ## 升级到最新
	$(COMPOSE) run --rm migrate up

migrate-down: ## 回滚最后一次
	$(COMPOSE) run --rm migrate down 1

migrate-new: ## 新建一对迁移文件 (NAME=blacklist_index)
	@test -n "$(NAME)" || (echo "用法: make migrate-new NAME=blacklist_index" && exit 1)
	@cd gateway/migrations && \
	  ts=$$(date +%s) && \
	  touch $${ts}_$(NAME).up.sql $${ts}_$(NAME).down.sql && \
	  echo "已建: $${ts}_$(NAME).{up,down}.sql"

seed: ## 灌种子数据
	$(COMPOSE) exec -T postgres psql -U $$POSTGRES_USER -d $$POSTGRES_DB < gateway/seed/seed.sql

# ── Gateway (Go) ────────────────────────────────────────────────────────
gateway-build: ## 本地编译 gateway
	cd gateway && go build -o bin/server ./cmd/server

gateway-run: ## 本地运行 gateway（依赖 pg/redis/minio 已起）
	cd gateway && go run ./cmd/server -addr=:8080

gateway-sqlc: ## 用 sqlc 生成 repo 代码
	cd gateway && sqlc generate -f internal/repo/sqlc.yaml

# ── AI (Python) ─────────────────────────────────────────────────────────
ai-install: ## 安装 Python 依赖
	cd ai && pip install -r requirements.txt

ai-run: ## 本地运行 AI
	cd ai && uvicorn app.main:app --host 0.0.0.0 --port 8090 --reload

# ── Quality ─────────────────────────────────────────────────────────────
fmt: ## 格式化
	cd gateway && gofmt -s -w .
	cd ai && python -m black app tests

lint:
	cd gateway && go vet ./...
	cd ai && python -m ruff check app

test: ## 跑测试
	cd gateway && go test ./...
	cd ai && python -m pytest

clean: ## 清理产物
	rm -rf gateway/bin
	cd ai && find . -type d -name __pycache__ -exec rm -rf {} +
