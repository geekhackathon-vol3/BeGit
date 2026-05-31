.PHONY: setup db-validate db-migrate-local

setup:
	git config core.hooksPath .githooks
	chmod +x .githooks/post-commit
	@echo "✅ git hooks の設定が完了しました"

db-validate:
	cd backend && npm run db:validate

db-migrate-local:
	cd backend && npm install && npm run db:migrate:local
