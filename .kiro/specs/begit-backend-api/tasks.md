# Implementation Plan

- [ ] 1. 基盤構築 — スキーマ・依存ライブラリ・モデル・暗号化・D1 クライアント
- [x] 1.1 groups テーブルに name / avatar_url カラムを追加するマイグレーションを作成する
  - `migrations/0002_add_groups_fields.sql` に `ALTER TABLE groups ADD COLUMN name TEXT NOT NULL DEFAULT ''; ALTER TABLE groups ADD COLUMN avatar_url TEXT;` を記述する
  - `wrangler d1 migrations apply begit-db` で適用できる状態にする
  - マイグレーション適用後に D1 の groups テーブルが name / avatar_url カラムを持つことを確認できる
  - _Requirements: 2.1, 2.2_

- [x] 1.2 golang.org/x/oauth2 依存と環境変数スキーマを追加する
  - `go get golang.org/x/oauth2` を実行して `go.mod` / `go.sum` を更新する
  - `cmd/server/main.go` に必要な環境変数一覧（`GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_WEBHOOK_SECRET`, `FIREBASE_SERVICE_ACCOUNT_JSON`, `DB_ENCRYPTION_KEY`, `CF_ACCOUNT_ID`, `D1_DATABASE_ID`, `CF_API_TOKEN`, `APP_BASE_URL`）を構造体 `Config` として定義する
  - 起動時に必須環境変数が未設定の場合はエラーログを出力して終了する
  - `go build ./...` が成功する
  - _Requirements: 7.2_

- [x] 1.3 全ドメインモデル型（User・Group・Sprint・Notification・Post・PostFeed・GroupMember）を定義する
  - `internal/model/models.go` に設計書の Go 構造体を実装する
  - ポインタ型フィールド（`*string`, `*int64` 等）を設計書の仕様どおりに定義する
  - `go build ./internal/model/...` が成功する
  - _Requirements: 7.6, 7.7_

- [x] 1.4 導出ノンス AES-GCM 暗号化・復号ユーティリティを実装する
  - `pkg/crypto/aes.go` に `Encryptor` インターフェースと実装を作成する
  - ノンス生成: `nonce = SHA-256(encryptionKey || plaintext)[:12]` （決定的）
  - 出力フォーマット: `hex(nonce) + ":" + hex(ciphertext)`
  - 同じ入力に対して `Encrypt` が常に同じ出力を返すこと（決定的暗号化の検証）
  - `Encrypt` → `Decrypt` で元の文字列が復元されることを確認できる
  - _Requirements: 7.1_

- [x] 1.5 Cloudflare D1 REST API クライアントを実装する
  - `pkg/d1/client.go` に `Query` / `Exec` メソッドを持つ `Client` インターフェースと実装を作成する
  - `POST https://api.cloudflare.com/client/v4/accounts/{account_id}/d1/database/{database_id}/query` を呼び出す
  - UNIQUE 制約違反（error_code に "UNIQUE constraint failed" を含む）は `ErrConstraintViolation`、空結果は `ErrNotFound` として返す
  - `Client.Query` がモック HTTP レスポンスに対して `[]map[string]interface{}` を正しく返すことを確認できる
  - _Requirements: 7.6_

- [ ] 2. 外部 API 連携クライアント — GitHub・FCM
- [x] 2.1 (P) GitHub REST API v3 クライアントを実装する
  - `pkg/github/client.go` に `ExchangeCode`, `GetUser`, `GetRepoInfo`, `GetCollaborators`, `RegisterWebhook`, `GetRecentCommits` を実装する
  - `ExchangeCode` は `POST https://github.com/login/oauth/access_token` に form データを送信し access_token を返す
  - `GetRecentCommits` は `GET /repos/{owner}/{repo}/commits?author={login}&per_page=5` → 最新コミット SHA で `GET /repos/{owner}/{repo}/commits/{sha}` を呼んで `CommitSummary` を返す（2 API calls）
  - GitHub API が 401 を返した場合は `ErrUnauthorized` を返す
  - 各メソッドがモック HTTP レスポンスに対して期待する型を返すことを確認できる
  - _Requirements: 1.1, 1.2, 2.3, 2.4, 4.1_
  - _Boundary: pkg/github_

- [x] 2.2 (P) FCM HTTP v1 クライアントを実装する
  - `pkg/fcm/client.go` に `SendToTokens(ctx, tokens []string, notification Notification) error` を実装する
  - `golang.org/x/oauth2/google` の `JWTConfigFromJSON` で Service Account JSON からアクセストークンを取得する
  - `POST https://fcm.googleapis.com/v1/projects/{project_id}/messages:send` を tokens 数分呼び出す
  - project_id は Service Account JSON の `project_id` フィールドから取得する
  - トークンリストが空の場合は API 呼び出しをスキップして正常終了する
  - _Requirements: 3.2_
  - _Boundary: pkg/fcm_

- [ ] 3. Repository 層 — 全 D1 テーブルへのアクセス実装
- [x] 3.1 (P) UserRepository を実装する
  - `GetByEncryptedToken`, `UpsertUser`, `GetByGitHubLogin` を D1 クライアント経由で実装する
  - `UpsertUser` は `INSERT OR REPLACE INTO users` で新規作成・既存更新の両方を処理する
  - `GetByEncryptedToken` が正しい `encrypted_access_token` に対してユーザーを返すことを確認できる
  - _Requirements: 1.3, 1.4, 1.7_
  - _Boundary: UserRepository_

- [x] 3.2 (P) GroupRepository を実装する
  - `ListByUserID`, `Create`, `GetByID`, `GetByRepoFullName`, `AddMember`, `BatchAddMembers`, `IsMember`, `GetMembers` を実装する
  - `ListByUserID` は `group_members JOIN groups` クエリで1回の D1 呼び出しで取得する
  - `GetByID` が存在しない group_id に対して `ErrNotFound` を返すことを確認できる
  - _Requirements: 2.1, 2.2, 2.4, 2.5, 2.6, 2.7_
  - _Boundary: GroupRepository_

- [x] 3.3 (P) SprintRepository を実装する
  - `GetOrCreateCurrentSprint` を実装する（`started_at <= now AND ends_at > now` で検索し存在しなければ INSERT）
  - `GetCurrentSprint` は現在アクティブなスプリントを返し、なければ `ErrNotFound` を返す
  - 同じ groupID に対して `GetOrCreate` を連続呼び出しした場合に1件のみ作成されることを確認できる
  - _Requirements: 3.1_
  - _Boundary: SprintRepository_

- [x] 3.4 (P) NotificationRepository と WebhookRepository を実装する
  - `NotificationRepository` に `Create` / `GetByID` を実装する
  - `Create` は `UNIQUE(sprint_id, sent_by)` 違反時に `ErrConstraintViolation` を返す
  - `WebhookRepository.InsertDelivery` は `INSERT INTO github_webhook_deliveries` を試みて UNIQUE 違反なら `isDuplicate=true, err=nil` を返す
  - 同じ delivery_id で `InsertDelivery` を2回呼んだとき `isDuplicate=true` が返ることを確認できる
  - _Requirements: 3.3, 3.8, 5.3_
  - _Boundary: NotificationRepository, WebhookRepository_

- [x] 3.5 (P) PostRepository と FCMTokenRepository を実装する
  - `PostRepository` に `Create`, `ListByGroupID`, `HasPostedInSprint`, `GetByUserAndNotification` を実装する
  - `FCMTokenRepository` に `Upsert`（`INSERT OR REPLACE INTO fcm_tokens`）と `GetTokensByGroupID` を実装する
  - `HasPostedInSprint` が posts テーブルの存在確認クエリで true/false を返すことを確認できる
  - _Requirements: 4.2, 4.3, 4.4, 6.1, 6.2_
  - _Boundary: PostRepository, FCMTokenRepository_

- [ ] 4. Service 層 — ビジネスロジック実装
- [x] 4.1 (P) GitHub OAuth 認証サービスを実装する
  - `AuthService.ExchangeCode` に GitHub コード交換 → ユーザー情報取得 → DB UPSERT → access_token 返却の一連のフローを実装する
  - `pkg/github.ExchangeCode` でアクセストークンを取得し、`pkg/github.GetUser` でユーザー情報を取得する
  - `pkg/crypto.Encrypt` でアクセストークンを暗号化してから `UserRepository.UpsertUser` を呼ぶ
  - GitHub が無効コードを返した場合は `ErrUnauthorized` を返す
  - `ExchangeCode` が有効コードで `AuthResult{User, Token}` を返すことを確認できる
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_
  - _Boundary: AuthService_

- [x] 4.2 (P) グループ管理サービスを実装する
  - `GroupService.CreateGroup` に Webhook 先行登録 → groups INSERT → group_members（owner）INSERT → コラボレーター自動追加の処理を実装する
  - `pkg/github.GetRepoInfo` でリポジトリオーナーの `avatar_url` を取得して groups に保存する
  - Webhook URL は `APP_BASE_URL + "/webhook/github"` で生成する
  - `pkg/github.RegisterWebhook` が失敗した場合は D1 INSERT を行わず `ErrExternalAPI` を返す
  - Webhook 登録成功後のみグループが D1 に作成されることを確認できる
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.8_
  - _Boundary: GroupService_

- [x] 4.3 (P) BeGit Time 通知サービスを実装する
  - `NotificationService.SendNotification` に現スプリント取得（または作成）→ 通知 INSERT → FCM 送信の処理を実装する
  - `SprintRepository.GetOrCreate` → `NotificationRepository.Create`（UNIQUE 違反で 409 Conflict を返す）
  - `FCMTokenRepository.GetTokensByGroupID` で全メンバーのトークンを取得し `pkg/fcm.SendToTokens` を呼ぶ
  - `GetNotificationStatus` でメンバーごとの投稿有無と時刻差から "On Time" / "Late" / "Missed" を算出する（1時間 = 3600秒の境界）
  - 同一スプリントで同一ユーザーが2回目の通知発行を試みると `ErrConflict` が返ることを確認できる
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8_
  - _Boundary: NotificationService_

- [x] 4.4 (P) 投稿・フィードサービスを実装する（ぼかし制御含む）
  - `PostService.CreatePost` に `pkg/github.GetRecentCommits` → posts INSERT の処理を実装する（GitHub API 失敗時は `ErrExternalAPI` を返す）
  - `ListPosts` で `SprintRepository.GetCurrentSprint` → `PostRepository.HasPostedInSprint` → 投稿リスト取得 → ぼかし制御を順に実行する
  - ぼかし制御: リクエストユーザーが未投稿の場合、他メンバーの `body`, `repo_full_name`, `latest_commit_message` フィールドを `nil` に設定する
  - `ListPosts` を呼ぶユーザーが未投稿の場合に他メンバーの sensitive フィールドが nil で返ることを確認できる
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_
  - _Boundary: PostService_

- [x] 4.5 (P) Webhook イベント処理と FCM トークン管理サービスを実装する
  - `WebhookService.ProcessWebhook` で `push` / `pull_request_review` イベントのリポジトリ名からグループを特定し、スプリント情報を更新する
  - 対応グループが見つからない場合は 200 OK で処理を終了する（エラーにしない）
  - `FCMTokenService.UpsertFCMToken` で `FCMTokenRepository.Upsert` を呼ぶ
  - `ProcessWebhook` が push イベントペイロードで正常完了（エラーなし）することを確認できる
  - _Requirements: 5.4, 5.5, 5.7, 6.1, 6.2, 6.3, 6.4_
  - _Boundary: WebhookService, FCMTokenService_

- [ ] 5. Handler 層・ミドルウェア — HTTP インターフェース
- [x] 5.1 Bearer 認証ミドルウェアとグループメンバー確認ミドルウェアを実装する
  - `BearerAuthMiddleware` で `Authorization: Bearer <token>` を抽出し `pkg/crypto.Encrypt(token)` → `UserRepository.GetByEncryptedToken` でユーザーを取得してコンテキストに `userID` を注入する
  - トークン不正・未設定の場合は `{"error": "unauthorized"}` と 401 を返す
  - `GroupMemberMiddleware` で URL パラメータの groupID と contextの userID から `GroupRepository.IsMember` を呼び、非メンバーに 403 を返す
  - 無効 Bearer トークンのリクエストが 401 を返すことを確認できる
  - _Requirements: 1.7, 2.6, 4.7_

- [x] 5.2 (P) AuthHandler と GroupHandler を実装する
  - `AuthHandler.ServeHTTP` で JSON ボディから `code` を取得し `AuthService.ExchangeCode` を呼んで `{"user": ..., "token": ...}` を返す
  - `GroupHandler` で `GET /groups`, `POST /groups`, `GET /groups/:id` の各メソッドを実装し GroupService に委譲する
  - 全レスポンスに `Content-Type: application/json` ヘッダーを付与する
  - バリデーションエラー（`repo_full_name` 欠損等）に 422 を返す
  - `POST /auth/github` が有効コードで 200 と `token` フィールドを返すことを確認できる
  - _Requirements: 1.5, 1.6, 2.1, 2.2, 2.5, 2.7, 7.3, 7.4, 7.5_
  - _Boundary: AuthHandler, GroupHandler_

- [x] 5.3 (P) PostHandler・WebhookHandler・NotificationHandler・FCMTokenHandler を実装する
  - `NotificationHandler` で `POST /groups/:id/notifications` と `GET /groups/:id/notifications/:nid` を実装し NotificationService に委譲する
  - `PostHandler` で `POST/GET /groups/:id/posts` を実装し PostService に委譲する
  - `WebhookHandler` で `X-Hub-Signature-256` を `hmac.Equal` で検証し失敗時は 403 を返す（Bearer 認証ミドルウェアを除外）、`WebhookRepository.InsertDelivery` で冪等性を確保してから `WebhookService.ProcessWebhook` を呼ぶ
  - `FCMTokenHandler` で `PUT /me/fcm-token` を実装し `fcm_token` フィールド欠損時は 400 を返す
  - `POST /webhook/github` が正しい HMAC で 200 を返し、誤った HMAC で 403 を返すことを確認できる
  - _Requirements: 3.1, 3.4, 4.3, 4.4, 5.1, 5.2, 5.3, 5.6, 6.3, 6.4, 7.3, 7.4, 7.5_
  - _Boundary: NotificationHandler, PostHandler, WebhookHandler, FCMTokenHandler_

- [ ] 6. サーバー統合・環境設定
- [x] 6.1 `cmd/server/main.go` にルーティング・DI・サーバー設定を完成させる
  - 全コンポーネントを依存関係順に初期化して DI で接続する（pkg → repository → service → handler）
  - Go 1.22 ServeMux で全エンドポイントを登録する（`/auth/github`, `/webhook/github`, `/groups`, `/groups/{id}`, `/groups/{id}/notifications`, `/groups/{id}/notifications/{nid}`, `/groups/{id}/posts`, `/me/fcm-token`）
  - BearerAuthMiddleware を `/webhook/github` と `/auth/github` を除く全ルートに適用する
  - `go build ./...` が成功し `go run ./cmd/server` でサーバーが起動することを確認できる
  - _Requirements: 7.6, 7.7_

- [x] 6.2 Cloudflare Workers 側のシークレット転送設定を行う（隣接変更）
  - `src/index.ts` の `fetch` ハンドラーで Workers Secrets と `[vars]` の値を `X-Internal-*` ヘッダーとして Container へのリクエストに追加する
  - Go Container の `main.go` で起動時に `os.Getenv("X_INTERNAL_*")` ではなく、最初のリクエストヘッダーから読む構成を取るか、または `wrangler.toml` の `[vars]` + シークレット binding を通じて環境変数として受け取る方法を採用する
  - `wrangler.toml` に `CF_ACCOUNT_ID`, `D1_DATABASE_ID`, `APP_BASE_URL` を `[vars]` セクションとして追加する
  - `wrangler dev` 起動後に Go Container が D1 に接続して `/groups` が 401 を返すことを確認できる
  - _Requirements: 7.2_

- [ ] 7. テスト — ユニットテスト・インテグレーションテスト
- [x] 7.1 (P) pkg/crypto の暗号化ユーティリティと BearerAuthMiddleware のユニットテストを書く
  - `pkg/crypto` のテスト: 暗号化の決定性（同一入力 → 同一出力）・可逆性（Encrypt→Decrypt）・異なる入力が異なる出力になることを検証する
  - `BearerAuthMiddleware` のテスト: 有効トークン/無効トークン/Authorization ヘッダー欠損の3ケースを検証する
  - `go test ./pkg/crypto/... ./internal/handler/...` がすべて PASS することを確認できる
  - _Requirements: 1.7, 7.1_
  - _Boundary: pkg/crypto, BearerAuthMiddleware_

- [x] 7.2 (P) 通知ステータス算出とぼかし制御ロジックのユニットテストを書く
  - `NotificationService.GetNotificationStatus` のテスト: On Time（通知後 59 分）/ Late（通知後 61 分）/ Missed（投稿なし）の境界値を検証する
  - `PostService.ListPosts` のぼかし制御テスト: 自分が未投稿 → 他メンバーの sensitive フィールドが nil、自分が投稿済み → 全フィールド公開 を検証する
  - `go test ./internal/service/...` がすべて PASS することを確認できる
  - _Requirements: 3.5, 3.6, 3.7, 4.4, 4.5_
  - _Boundary: NotificationService, PostService_

- [x] 7.3 Webhook Handler の HMAC 検証・冪等性インテグレーションテストを書く
  - `POST /webhook/github` に正しい `X-Hub-Signature-256` を付与したリクエストが 200 を返すことを検証する
  - 誤った署名が 403 を返すことを検証する
  - 同じ `X-GitHub-Delivery` 値で2回リクエストを送ると2回目も 200 を返して処理がスキップされることを検証する（5.3 冪等性）
  - `go test ./internal/handler/...` がすべて PASS することを確認できる
  - _Requirements: 5.1, 5.2, 5.3_
