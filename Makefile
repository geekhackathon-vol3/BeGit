.PHONY: setup terraform-apply deploy secrets-init warmup verify-local smoke-test

WORKERS_URL ?= https://begit.118029-ichikama.workers.dev

setup:
	git config core.hooksPath .githooks
	chmod +x .githooks/post-commit
	@echo "✅ git hooks の設定が完了しました"

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
