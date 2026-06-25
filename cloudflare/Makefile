.PHONY: help setup db-create db-migrate db-seed secret deploy-all dev-auth dev-workout dev-user dev-menu dev-export

WORKERS := auth workout user menu export
DOMAIN  ?= workout.example.com   # 自分のドメインに変更

help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ===== セットアップ =====

setup: ## 全 Worker の npm install を実行
	@for w in $(WORKERS); do \
		echo "→ install workers/$$w"; \
		cd workers/$$w && npm install && cd ../..; \
	done

# ===== Cloudflare D1 =====

db-create: ## D1 データベースを作成 (初回のみ)
	wrangler d1 create workout-memo
	@echo "↑ 上記の database_id を全 wrangler.toml の database_id に記入してください"

db-migrate: ## D1 にスキーマを適用
	wrangler d1 execute workout-memo --file=d1/schema.sql

db-seed: ## D1 にデフォルト種目を投入
	wrangler d1 execute workout-memo --file=d1/seed_exercises.sql

db-query: ## D1 の SQL をインタラクティブに実行 (例: make db-query SQL="SELECT * FROM users")
	wrangler d1 execute workout-memo --command="$(SQL)"

# ===== シークレット設定 =====

secret: ## JWT_SECRET を全 Worker に設定 (対話型)
	@SECRET=$$(openssl rand -hex 32); \
	for w in $(WORKERS); do \
		echo "$$SECRET" | wrangler secret put JWT_SECRET --name workout-$$w; \
	done
	@echo "✅ JWT_SECRET を全 Worker に設定しました"

# ===== デプロイ =====

deploy-all: ## 全 Worker を本番デプロイ (auth → workout → user → menu → export の順)
	@for w in $(WORKERS); do \
		echo "→ deploy workers/$$w"; \
		cd workers/$$w && npm run deploy && cd ../..; \
	done
	@echo "✅ デプロイ完了"
	@echo "Pages: https://dash.cloudflare.com → Pages → workout-memo-frontend にビルドを追加"

deploy-%: ## 指定 Worker だけデプロイ (例: make deploy-auth)
	cd workers/$* && npm run deploy

# ===== ローカル開発 =====

dev-auth: ## auth Worker をローカル起動 (http://localhost:8787)
	cd workers/auth && npm run dev

dev-workout:
	cd workers/workout && npm run dev

dev-user:
	cd workers/user && npm run dev

dev-menu:
	cd workers/menu && npm run dev

dev-export:
	cd workers/export && npm run dev

# ===== Cloudflare Pages =====

pages-deploy: ## index.html を Cloudflare Pages にデプロイ
	wrangler pages deploy ../ \
		--project-name workout-memo-frontend \
		--branch main \
		--commit-dirty=true

# ===== R2 =====

r2-create: ## CSV エクスポート用 R2 バケット作成
	wrangler r2 bucket create workout-memo-exports
