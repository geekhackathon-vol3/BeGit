# impl — begit-backend-api

**実行日時**: 2026-06-01T01:00:00+09:00
**フェーズ**: impl（サブエージェント）

## サマリー

- tasks.md 全 18 サブタスク（1.1〜7.3）を TDD サイクルで実装完了。全9パッケージのテストが PASS
- Clean Architecture（handler → service → repository → pkg）の依存方向を厳守し、設計書の全インターフェースを実装
- 導出ノンス AES-GCM 暗号化（決定的）、D1 REST API クライアント、GitHub/FCM 外部 API クライアントを pkg 層として実装
- Webhook HMAC-SHA256 検証・冪等性制御、Bearer 認証ミドルウェア、ぼかし制御（PostService.ListPosts）のロジックを実装
- Workers Secrets を `X-Internal-*` ヘッダーで Container に転送する src/index.ts の隣接変更も完了

## 成果物

### 新規作成ファイル
- `backend/migrations/0002_add_groups_fields.sql`
- `backend/internal/model/models.go` / `models_test.go`
- `backend/internal/repository/errors.go`
- `backend/internal/repository/user_repository.go` / `_test.go`
- `backend/internal/repository/group_repository.go` / `_test.go`
- `backend/internal/repository/sprint_repository.go` / `_test.go`
- `backend/internal/repository/notification_repository.go` / `_test.go`
- `backend/internal/repository/webhook_repository.go`
- `backend/internal/repository/post_repository.go` / `_test.go`
- `backend/internal/repository/fcm_token_repository.go`
- `backend/internal/service/errors.go`
- `backend/internal/service/auth_service.go` / `_test.go`
- `backend/internal/service/group_service.go` / `_test.go`
- `backend/internal/service/notification_service.go` / `_test.go`
- `backend/internal/service/post_service.go` / `_test.go`
- `backend/internal/service/webhook_service.go` / `_test.go`
- `backend/internal/service/fcm_token_service.go`
- `backend/internal/handler/middleware.go` / `_test.go`
- `backend/internal/handler/auth.go` / `_test.go`
- `backend/internal/handler/groups.go`
- `backend/internal/handler/notifications.go`
- `backend/internal/handler/posts.go`
- `backend/internal/handler/webhook.go` / `_test.go`
- `backend/internal/handler/fcm_token.go`
- `backend/pkg/crypto/aes.go` / `_test.go`
- `backend/pkg/d1/client.go` / `_test.go`
- `backend/pkg/github/client.go` / `_test.go`
- `backend/pkg/fcm/client.go` / `_test.go`

### 更新ファイル
- `backend/cmd/server/main.go` — Config 構造体・DI・全エンドポイントルーティング
- `backend/cmd/server/main_test.go` — 必須環境変数バリデーションテスト
- `backend/src/index.ts` — X-Internal-* ヘッダー転送追加
- `backend/wrangler.toml` — [vars] セクション追加
- `backend/go.mod` / `go.sum` — golang.org/x/oauth2 依存追加

## 主要な決定事項 / 発見事項

1. **go get によるバージョン更新**: `go get golang.org/x/oauth2` により go 1.22 → 1.25.0 に自動更新された。ランタイム互換性に問題なし
2. **groupCreateInput の公開**: サービス層から GroupRepository.Create を呼ぶために `groupCreateInput` → `GroupCreateInput` に変更（小文字→大文字）
3. **NotificationService の複数コンストラクタ**: SendNotification（FCM 依存）と GetNotificationStatus（GroupRepo/PostRepo 依存）が分離しているため、コンストラクタを3種類用意
4. **PostService.CreatePost の GitHub ログイン**: design.md にない `GitHubLogin` と `RepoFullName` フィールドを CreatePostRequest に追加（GitHub API 呼び出しに必要）
5. **main.go の初期化遅延**: Workers Container では環境変数が空の場合、`sync.Once` を使って最初のリクエスト時に X-Internal-* ヘッダーから設定を補完する戦略を採用

## 次フェーズへの引き継ぎ事項

- 全 18 サブタスク完了、未実装タスクなし
- 全テスト PASS（9パッケージ、38テスト関数）
- `wrangler dev` による実機テストは未実施（D1・GitHub・FCM 実接続が必要）
- wrangler.toml の `CF_ACCOUNT_ID` と `APP_BASE_URL` は空文字のままのため、本番デプロイ前に設定が必要
- Workers Secrets（`GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, etc.）の `wrangler secret put` 設定が必要
