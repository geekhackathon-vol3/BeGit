# Research & Design Decisions

---
**Purpose**: Discovery findings and architectural rationale for the begit-backend-api design.
---

## Summary

- **Feature**: `begit-backend-api`
- **Discovery Scope**: New Feature（greenfield Go API）with Complex Integration（GitHub OAuth / FCM / Webhook）
- **Key Findings**:
  - Cloudflare Workers Container の D1 バインディングは Worker 側にのみ存在するため、Go Container は D1 REST API 経由でデータアクセスを行う
  - AES-GCM で Bearer トークンを検索可能にするため「導出ノンス（derived nonce）」による決定的暗号化を採用する
  - `src/index.ts` にシークレット転送コードを追加する隣接変更が必要（routing logic の変更ではない）
  - `groups` テーブルに `name` / `avatar_url` カラムが存在しないため `0002_add_groups_fields.sql` マイグレーションが必要
  - GitHub Webhook 登録時に公開 URL（`APP_BASE_URL`）環境変数が必要
  - FCM HTTP v1 API は Service Account JWT を要し、`golang.org/x/oauth2/google` で生成する

## Research Log

### D1 アクセス方法の調査

- **Context**: Go Workers Container は Cloudflare D1 バインディングに直接アクセスできない
- **Sources Consulted**: Cloudflare Workers Containers ドキュメント（training knowledge）
- **Findings**:
  - D1 バインディングは Worker (TypeScript) 側の `Env` にのみ存在する
  - Container プロセスはポート 8080 で HTTP リクエストを受け取るだけ
  - D1 REST API エンドポイント: `POST https://api.cloudflare.com/client/v4/accounts/{account_id}/d1/database/{database_id}/query`
  - 認証: `Authorization: Bearer {cf_api_token}`
  - リクエスト: `{ "sql": "...", "params": [...] }`
  - レスポンス: `{ "result": [{ "results": [...], "success": true, "meta": {...} }], "success": true }`
- **Implications**: `pkg/d1` パッケージを実装し、環境変数 `CF_ACCOUNT_ID`, `D1_DATABASE_ID`, `CF_API_TOKEN` を Container に渡す必要がある

### Bearer トークン認証ルックアップの調査

- **Context**: AES-GCM はランダムノンスのため同じ平文が毎回異なる暗号文になり、DB 検索に使えない
- **Findings**:
  - 解決策 A: `token_hash = SHA-256(access_token)` カラムを追加（スキーマ変更必要）
  - 解決策 B: `nonce = SHA-256(key || plaintext)[:12]` の導出ノンスで決定的暗号化（スキーマ変更不要）
  - 解決策 B は同一平文→同一暗号文を保証。アクセストークンは十分にランダムなため nonce 再利用のリスクは許容範囲
- **Selected**: 解決策 B（導出ノンス）を採用。スキーマ変更なし、実装シンプル
- **Implications**: `pkg/crypto` の `Encrypt(plaintext)` は決定的であり、Auth Middleware で `Encrypt(bearer_token)` → DB 検索が可能

### シークレット注入方法の調査

- **Context**: Go Container が `GITHUB_CLIENT_SECRET`, `DB_ENCRYPTION_KEY` 等の Workers Secrets にアクセスする方法
- **Findings**:
  - Cloudflare Workers Container は Docker プロセスへの env var 注入が wrangler.toml の `[vars]` でサポートされているが、Secrets（暗号化値）は直接 Container env には届かない
  - Worker が `request.headers.set('X-Internal-Secret', env.MY_SECRET)` でリクエストヘッダーに追加し、Container で読む方法が現実的
  - セキュリティ上の懸念: 内部ヘッダーが外部露出しないよう Worker 側で `X-Internal-*` ヘッダーを上書き（外部からのヘッダーを除去）
  - 設計上の注意: 起動時に `main.go` で一度 `os.Getenv` で読むのが推奨パターン
- **Implications**: `src/index.ts` に数行の追加が必要（このスペックの boundary 外だが隣接変更として記録）

### GitHub Commit 取得 API の調査

- **Context**: POST /groups/:id/posts でユーザーの直近コミット情報を自動取得する（Req 4.1）
- **Sources**: GitHub REST API v3 ドキュメント
- **Findings**:
  - `GET /repos/{owner}/{repo}/commits?author={login}&per_page=5` で直近コミット一覧
  - レスポンスにはコミット件数・メッセージが含まれるが additions/deletions は含まれない
  - `GET /repos/{owner}/{repo}/commits/{sha}` で個別コミットの stats を取得可能
  - Rate limit: GitHub OAuth token で 5000 req/hour
- **Implications**: 2回の GitHub API 呼び出しが必要（リスト取得 + 最新コミット詳細）。post 作成ごとに API 呼び出しが発生するため rate limit に注意

### FCM HTTP v1 API 認証の調査

- **Context**: `FIREBASE_SERVICE_ACCOUNT_JSON` を使った FCM 送信
- **Findings**:
  - FCM HTTP v1 エンドポイント: `POST https://fcm.googleapis.com/v1/projects/{project_id}/messages:send`
  - 認証: Google OAuth2 Service Account で `https://www.googleapis.com/auth/firebase.messaging` スコープのアクセストークンを取得
  - `golang.org/x/oauth2/google` パッケージの `google.JWTConfigFromJSON` が最も簡潔
  - アクセストークンの有効期限は 1 時間。`TokenSource` 実装が自動的にリフレッシュ
- **Implications**: `go.mod` に `golang.org/x/oauth2` 依存を追加。FCM Client は `TokenSource` をキャッシュして再利用

### groups テーブルのスキーマギャップ

- **Context**: Req 2.1 / 2.2 が `name`, `avatar_url` フィールドを必要とするが `0001_initial.sql` の groups テーブルにない
- **Findings**:
  - `groups` テーブルに `name TEXT` と `avatar_url TEXT` カラムが欠如
  - SQLite の ALTER TABLE ADD COLUMN はデフォルト値なしの NOT NULL を許可しない
  - `avatar_url` はグループ作成時に GitHub の `GET /repos/{owner}/{repo}` から `owner.avatar_url` を取得して保存
- **Implications**: `migrations/0002_add_groups_fields.sql` で `ALTER TABLE groups ADD COLUMN name TEXT NOT NULL DEFAULT ''; ALTER TABLE groups ADD COLUMN avatar_url TEXT;` を追加

## Architecture Pattern Evaluation

| オプション | 説明 | 強み | リスク | 備考 |
|--------|-----|------|------|------|
| Clean Architecture 3層 | handler → service → repository | 依存方向が明確、テスト容易、steering 準拠 | ボイラープレート多め | 採用。steering の方針と完全一致 |
| net/http + Go 1.22 ServeMux | 標準ライブラリのみ | 外部依存なし、パターンルーティング対応（:id 等） | 高度なミドルウェアチェーン実装が手動 | 採用。Go 1.22 の enhanced routing で必要十分 |
| chi / gorilla/mux | サードパーティルーター | ミドルウェアチェーンが便利 | 外部依存追加 | 不採用。net/http で十分 |
| D1 REST API | CF API 経由 DB アクセス | Container から利用可能、SQL 互換 | RTT オーバーヘッド（約 50-100ms / query） | 採用。代替なし |

## Design Decisions

### Decision: 導出ノンスによる決定的 AES-GCM

- **Context**: Bearer トークン（GitHub access_token）を DB のencrypted_access_token で検索する必要がある
- **Alternatives Considered**:
  1. token_hash カラム追加（SHA-256 ハッシュで検索）
  2. 導出ノンス AES-GCM（nonce = HKDF/SHA-256 派生の 12 byte）
- **Selected Approach**: 導出ノンス: `nonce = SHA-256(encryptionKey || plaintext)[:12]`, ciphertext = AES-GCM(key, nonce, plaintext)
- **Rationale**: スキーマ変更なし。アクセストークンは GitHub が生成するランダム文字列のため nonce 重複は実質ゼロ
- **Trade-offs**: ✅ シンプル ✅ スキーマ変更不要 ⚠️ ノンス固定化はセキュリティ理論上非推奨だが実用上問題なし
- **Follow-up**: 本番環境ではトークンローテーション時に再暗号化不要か確認

### Decision: Workers → Container シークレット転送

- **Context**: Go Container に Workers Secrets を渡す方法
- **Alternatives Considered**:
  1. `src/index.ts` で `X-Internal-*` ヘッダーとして注入
  2. wrangler.toml `[vars]` に平文で記載（Secrets は不可）
  3. D1 proxy: Worker が `/internal/d1` を提供
- **Selected Approach**: Option 1（ヘッダー注入）
- **Rationale**: Workers Secrets の暗号化保護を維持しつつ Container へ安全に転送できる最小変更
- **Trade-offs**: ✅ 最小変更 ⚠️ ヘッダーは内部通信のみ（外部には届かない Cloudflare Container ネットワーク）

### Decision: グループ作成時の Webhook 先行登録

- **Context**: Req 2.8「Webhook 登録失敗時にグループ作成をロールバック」
- **Selected Approach**: Webhook 登録を先に実行し、成功時のみ D1 INSERT を実行
- **Rationale**: D1 のロールバックを不要にする。Webhook 成功後に D1 INSERT が失敗する場合は孤立 Webhook が残るが、ハッカソンスコープでは許容
- **Trade-offs**: ✅ トランザクション不要 ⚠️ D1 INSERT 失敗時の孤立 Webhook（既知の制限として文書化）

## Risks & Mitigations

- D1 REST API の RTT（50-100ms/query）がレスポンスタイムを増加させる — N+1 クエリを避け、GROUP BY / JOIN で1クエリに集約
- GitHub API rate limit (5000 req/hour per token) — コミット取得の API 呼び出し最小化（2 calls per post）
- Workers Container のコールドスタート（Durable Object sleep after 10m）— デモ前のウォームアップ推奨
- FCM 送信失敗（トークン無効等）— 送信エラーをログに残し、通知 INSERT 自体は成功扱い（非同期的な失敗は許容）

## References

- [Cloudflare D1 REST API](https://developers.cloudflare.com/d1/worker-api/) — D1 HTTP API 仕様
- [FCM HTTP v1 API](https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages) — FCM 送信エンドポイント
- [GitHub REST API - Commits](https://docs.github.com/en/rest/commits/commits) — コミット取得 API
- [golang.org/x/oauth2/google](https://pkg.go.dev/golang.org/x/oauth2/google) — Service Account JWT 生成
