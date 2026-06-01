.PHONY: setup dev terraform-apply deploy secrets-init warmup verify-local smoke-test dev-db-create deploy-dev seed-dev openapi openapi-sync

SWAG_VERSION ?= v2.0.0-rc5

WORKERS_URL ?= https://begit.118029-ichikama.workers.dev
DEV_URL ?= https://begit-dev.118029-ichikama.workers.dev

setup:
	git config core.hooksPath .githooks
	chmod +x .githooks/post-commit
	@echo "✅ git hooks の設定が完了しました"

# swag (gin のアノテーション) から OpenAPI 3.1 仕様を再生成する。
# 生成物: backend/docs/swagger.json, backend/docs/swagger.yaml
# swag が未インストールなら自動で取得する。
openapi:
	@command -v swag >/dev/null 2>&1 || go install github.com/swaggo/swag/v2/cmd/swag@$(SWAG_VERSION)
	cd backend && swag init -g cmd/server/main.go -o docs --ot json,yaml --parseInternal --v3.1
	@echo "✅ OpenAPI 3.1 仕様を backend/docs/ に生成しました"

# OpenAPI 仕様を再生成し、iOS (swift-openapi-generator) のターゲットへ配布する。
# iOS 側はソースフォルダ内の openapi.yaml をビルド時に読んで型/クライアントを生成する。
IOS_OPENAPI_DEST ?= ios/BeGit/BeGit/openapi.yaml
openapi-sync: openapi
	cp backend/docs/swagger.yaml $(IOS_OPENAPI_DEST)
	@echo "✅ OpenAPI 仕様を $(IOS_OPENAPI_DEST) へ同期しました（iOS をリビルドすると型が追随します）"

# ローカル開発サーバー起動（.envrc の変数を使用）
# 必要な環境変数: TF_VAR_cloudflare_api_token, GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET
dev:
	@[ -n "$$TF_VAR_cloudflare_api_token" ] || { echo "❌ TF_VAR_cloudflare_api_token が未設定です（.envrc を確認）"; exit 1; }
	@[ -n "$$GITHUB_CLIENT_ID" ]            || { echo "❌ GITHUB_CLIENT_ID が未設定です（.envrc を確認）"; exit 1; }
	@[ -n "$$GITHUB_CLIENT_SECRET" ]        || { echo "❌ GITHUB_CLIENT_SECRET が未設定です（.envrc を確認）"; exit 1; }
	@echo "🚀 BeGit API 起動中 → http://localhost:8080"
	CF_API_TOKEN="$$TF_VAR_cloudflare_api_token" \
	CF_ACCOUNT_ID="c53d3c6ca02ae31a86aa3bf8fcbe5e55" \
	D1_DATABASE_ID="9629cd07-59c2-4b35-a795-d91a6b43fd02" \
	DB_ENCRYPTION_KEY="0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20" \
	APP_BASE_URL="http://localhost:8080" \
	GITHUB_CLIENT_ID="$$GITHUB_CLIENT_ID" \
	GITHUB_CLIENT_SECRET="$$GITHUB_CLIENT_SECRET" \
	GITHUB_WEBHOOK_SECRET="dev-webhook-secret" \
	FIREBASE_SERVICE_ACCOUNT_JSON='{"type":"service_account","project_id":"dummy"}' \
	go run -C backend ./cmd/server

# Task 4.1: Terraform apply + wrangler.toml database_id 自動更新
terraform-apply:
	terraform -chdir=infra/terraform apply
	@D1_ID=$$(terraform -chdir=infra/terraform output -raw d1_database_id) && \
	sed "s|database_id = \".*\"|database_id = \"$$D1_ID\"|" backend/wrangler.toml > backend/wrangler.toml.tmp && \
	mv backend/wrangler.toml.tmp backend/wrangler.toml && \
	echo "✅ backend/wrangler.toml の database_id を更新しました: $$D1_ID"

# Task 4.2: Docker build → wrangler deploy → D1 migration
deploy:
	docker build --platform linux/amd64 -t begit-api ./backend && \
	cd backend && npx wrangler deploy && \
	npx wrangler d1 migrations apply begit-db

# ── dev 環境（フロント共有用）─────────────────────────────────────────
# dev D1 を作成（初回のみ）。出力された database_id を
# backend/wrangler.toml の該当フィールドに貼り付ける。
dev-db-create:
	cd backend && npx wrangler d1 create begit-db-dev
	@echo ""
	@echo "👉 出力された database_id を backend/wrangler.toml の以下の2箇所に貼り付けてください:"
	@echo "   - [env.dev.vars].D1_DATABASE_ID"
	@echo "   - [[env.dev.d1_databases]].database_id"
	@echo "👉 次に: npx wrangler secret put CF_API_TOKEN --env dev （cd backend で実行）"

# dev Worker(begit-dev) をデプロイ（DEV_MODE=true）。Docker build → deploy → dev D1 migration
deploy-dev:
	docker build --platform linux/amd64 -t begit-api ./backend && \
	cd backend && npx wrangler deploy --env dev && \
	npx wrangler d1 migrations apply begit-db-dev --env dev --remote

# dev API にシードデータを投入（公開 API 経由で作成。スモークテスト兼用）
seed-dev:
	DEV_URL="$(DEV_URL)" ./scripts/seed-dev.sh

# Task 4.3: シークレット登録手順表示 + コンテナウォームアップ
secrets-init:
	@echo "以下のコマンドを順番に実行してシークレットを登録してください:"
	@echo ""
	@echo "  npx wrangler secret put GITHUB_CLIENT_SECRET"
	@echo "  npx wrangler secret put GITHUB_WEBHOOK_SECRET"
	@echo "  npx wrangler secret put FIREBASE_SERVICE_ACCOUNT_JSON"
	@echo "  npx wrangler secret put DB_ENCRYPTION_KEY"
	@echo ""
	@echo "各コマンド実行後、プロンプトに値を入力してください。"

warmup:
	curl $(WORKERS_URL)/

# Task 5.1: ローカル開発環境での接続確認
verify-local:
	@echo "=== ローカル D1 接続確認 ==="
	cd backend && npx wrangler d1 execute begit-db --local --command "SELECT 1"
	@echo "✅ ローカル D1 接続: OK"

# Task 5.2: デプロイ後スモークテスト
smoke-test:
	@echo "=== シークレット登録確認 ==="
	cd backend && npx wrangler secret list
	@echo "=== デプロイ済み Workers ヘルスチェック ==="
	curl -sf $(WORKERS_URL)/ || { echo "⚠️  Workers レスポンスなし（未デプロイの可能性あり）"; exit 1; }
	@echo "=== D1 マイグレーション適用確認 ==="
	cd backend && npx wrangler d1 execute begit-db --remote --command "SELECT name FROM sqlite_master WHERE type='table'"
	@echo "=== Terraform 差分確認 ==="
	terraform -chdir=infra/terraform plan
