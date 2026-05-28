.PHONY: setup

setup:
	git config core.hooksPath .githooks
	chmod +x .githooks/post-commit
	@echo "✅ git hooks の設定が完了しました"
