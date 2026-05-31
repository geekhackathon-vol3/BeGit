.PHONY: setup terraform-apply deploy secrets-init warmup

WORKERS_URL ?= https://begit.workers.dev

setup:
	git config core.hooksPath .githooks
	chmod +x .githooks/post-commit
	@echo "✅ git hooks の設定が完了しました"

# Task 4.1: Terraform apply + wrangler.toml database_id 自動更新
terraform-apply:
	terraform -chdir=infra/terraform apply
	@D1_ID=$$(terraform -chdir=infra/terraform output -raw d1_database_id) && \
	sed -i '' "s|database_id = \".*\"|database_id = \"$$D1_ID\"|" backend/wrangler.toml && \
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
