# ギャップ分析レポート

**機能**: begit-backend-api  
**日付**: 2026-06-01  
**フェーズ**: Requirements → Design

---

## 分析サマリー

- **現状**: Cloudflare Workers エントリーポイント（`src/index.ts`）と D1 スキーマ（`migrations/0001_initial.sql`）は整備済みだが、Go バックエンドは `cmd/server/main.go` のスタブ1ファイルのみ。`internal/` / `pkg/` 層は未作成。
- **最大の課題**: Workers Container 内 Go サーバーから D1 / R2 へのアクセス方式が未確定（要調査）。FCM HTTP v1 API の認証フロー（サービスアカウント JWT）も設計が必要。
- **スキーマギャップ**: `groups` テーブルに `name` カラムが存在しない（要マイグレーション追加）。
- **推奨アプローチ**: Option B（新規コンポーネント群を新設） — 要件が明示する `handler → service → repository` 3 層 + `pkg/` 外部連携層をゼロから構築する。
- **工数 / リスク**: L（1〜2 週間） / リスク：High（D1 アクセス方式未確定が全 DB 操作に影響）

---

## 1. 現状調査

### 1-1. 既存ファイルマップ

| パス | 状態 | 内容 |
|---|---|---|
| `src/index.ts` | **完成** | Workers エントリーポイント。`BeGitAPI` Container (DO) 定義、全リクエストを `getContainer(...).fetch()` へ転送 |
| `wrangler.toml` | **完成** | Container / D1 / R2 / Secrets バインディング設定済み |
| `Dockerfile` | **完成** | linux/amd64 向け Go マルチステージビルド |
| `go.mod` | **スタブ** | `module github.com/irj0927/begit / go 1.22` のみ。依存パッケージ未記載 |
| `cmd/server/main.go` | **スタブ** | `GET /` に "BeGit API" を返すだけ。ルーター・ミドルウェア未実装 |
| `migrations/0001_initial.sql` | **完成** | 全テーブル定義済み（後述のギャップあり） |
| `internal/` | **未作成** | handler / service / repository 層なし |
| `pkg/` | **未作成** | github / fcm クライアントなし |

### 1-2. D1 スキーマ現状

全テーブルは定義済みだが **1 件のギャップ**を確認：

| テーブル | 問題 |
|---|---|
| `groups` | **`name` カラム欠如** — 要件では `GET /groups` が `name` を返し `POST /groups` が `name` を受け取る。スキーマには `repo_full_name` のみ存在 |
| `groups` | `avatar_url` なし — `GET /groups` の `avatar_url` は `users.avatar_url`（owner JOIN）で対応可能なため追加不要 |

### 1-3. Workers エントリーポイントの Env 定義

`src/index.ts` の `Env` インターフェースから確認できるシークレット：

```typescript
GITHUB_CLIENT_SECRET: string
GITHUB_WEBHOOK_SECRET: string
FIREBASE_SERVICE_ACCOUNT_JSON: string
DB_ENCRYPTION_KEY: string
```

Go サーバーがこれらにアクセスする方法は未実装（Workers が環境変数として注入するか、HTTP ヘッダー経由かを設計で決定する必要あり）。

---

## 2. 要件フィージビリティ分析

### 2-1. 要件 → 技術要素マッピング

| 要件 | 必要な技術要素 | 状態 |
|---|---|---|
| **R1: GitHub OAuth** | POST /auth/github、GitHub token エンドポイント呼び出し、AES-GCM 暗号化、D1 UPSERT | **Missing** |
| **R1: Bearer 認証ミドルウェア** | 全ハンドラー共通 middleware、D1 token lookup | **Missing** |
| **R2: グループ管理** | CRUD for groups / group_members、GitHub Webhook 登録 API 呼び出し、GitHub collaborators API | **Missing** |
| **R3: 通知発行** | sprints 当日レコード取得/生成、notifications INSERT、FCM HTTP v1 API 呼び出し | **Missing** |
| **R3: ステータス算出** | On Time / Late / Missed ロジック（時刻比較）| **Missing** |
| **R4: 投稿** | GitHub commits API（最近のコミット情報取得）、posts INSERT | **Missing** |
| **R4: ぼかしフィード** | リクエストユーザーの当スプリント投稿有無でレスポンスを加工 | **Missing** |
| **R5: Webhook 受信** | HMAC-SHA256 署名検証、冪等 INSERT（`github_webhook_deliveries`）| **Missing** |
| **R6: FCM トークン管理** | fcm_tokens UPSERT | **Missing** |
| **R7: セキュリティ共通** | AES-GCM 実装、Secrets 注入、JSON error 統一、layer 依存方向 | **Missing** |

### 2-2. 技術決定事項（確定済み）

| # | 内容 | 決定 |
|---|---|---|
| **RN-1** | **Workers Container から D1 へのアクセス方式** | **Workers プロキシ** — `src/index.ts` の Workers スクリプトが D1 クエリを仲介し、Go コンテナは Workers 経由で DB 操作を行う |
| **RN-2** | **FCM HTTP v1 API 認証** | `golang.org/x/oauth2/google` で Service Account JWT を生成し Bearer トークンを取得 |
| **RN-3** | **GitHub "最近のコミット" 取得 API** | `/repos/{owner}/{repo}/commits?author={login}&since=<今日0時>` で当日コミットを取得。変更行数は各コミットの `stats` フィールドを集計 |
| **RN-4** | **HTTP ルーター** | **Gin** (`github.com/gin-gonic/gin`) を採用。`:id` などパスパラメータと middleware チェーンを活用 |

### 2-3. 未解決事項

| # | 内容 | 影響 |
|---|---|---|
| **RN-5** | **Workers プロキシの具体的な通信プロトコル** — Go サーバーへの転送前後に D1 操作を挟む方法（例: 専用エンドポイントを内部に立てる / リクエストヘッダーに D1 結果を注入するなど）の詳細は設計フェーズで確定 | `src/index.ts` の改修範囲と repository 層のインターフェース設計に影響 |
| **RN-6** | **R2 アクセス方式** — 将来の写真アップロードで必要。今 Spec スコープ外だが、設計で触れておく | スコープ外 |

---

## 3. 実装アプローチ評価

### Option A: 既存コンポーネントを拡張

**対象ファイル**: `cmd/server/main.go` に全ロジックを追加  
**評価**: ❌ 非推奨。1 ファイルに全機能を詰め込むと要件 R7 の「layer 依存方向厳守」に直ちに違反する。スタブファイルにルーティングのみ残す変更は必要だが、ここへのロジック集約は却下。

### Option B: 新規コンポーネント群を新設（推奨）

要件 R7 で明示された構成通りに全層をゼロ新設：

```
backend/
├── cmd/server/main.go          ← ルーター + ミドルウェア登録のみに改修
├── internal/
│   ├── handler/
│   │   ├── auth.go             ← POST /auth/github
│   │   ├── groups.go           ← GET/POST /groups, GET /groups/:id
│   │   ├── notifications.go    ← POST/GET /groups/:id/notifications/:nid
│   │   ├── posts.go            ← POST/GET /groups/:id/posts
│   │   ├── webhook.go          ← POST /webhook/github
│   │   └── fcm.go              ← PUT /me/fcm-token
│   ├── service/
│   │   ├── auth.go
│   │   ├── groups.go
│   │   ├── notifications.go
│   │   ├── posts.go
│   │   ├── webhook.go
│   │   └── fcm.go
│   └── repository/
│       ├── users.go
│       ├── groups.go
│       ├── notifications.go
│       ├── posts.go
│       ├── sprints.go
│       ├── fcm_tokens.go
│       └── webhook.go
└── pkg/
    ├── github/client.go        ← GitHub REST API クライアント
    ├── fcm/client.go           ← FCM HTTP v1 クライアント
    └── crypto/aes.go           ← AES-GCM 暗号化/復号
```

**必要な Go 依存パッケージ候補（go.mod 追加）**:

| パッケージ | 用途 |
|---|---|
| `github.com/go-chi/chi/v5` または `net/http` + `gorilla/mux` | URL パラメータ（`:id`）付きルーティング |
| `golang.org/x/oauth2/google` | FCM 用 Service Account JWT（RN-2 確認後） |
| 標準 `crypto/aes` / `crypto/cipher` | AES-GCM（外部依存不要） |
| 標準 `crypto/hmac` / `crypto/sha256` | Webhook 署名検証（外部依存不要） |

**Trade-offs**:
- ✅ 要件 R7 の layer 構成・依存方向を完全に遵守
- ✅ 各層が独立しており単体テストが容易
- ✅ 新規追加のため既存機能への影響なし
- ❌ ファイル数が多く、最初の骨格作成コストが高い
- ❌ D1 アクセス方式（RN-1）が未確定のため repository 層の実装が始められない

### Option C: ハイブリッド

`main.go` をルーターとして改修し、handler/service/repository を段階的に追加していく。実質 Option B と同一の成果物になるため、Option B として統一する。

---

## 4. 複雑度・リスク評価

| 要件 | 工数 | リスク | 理由 |
|---|---|---|---|
| R1: GitHub OAuth + 暗号化 | M | Medium | OAuth フローは既知。AES-GCM は標準ライブラリで実装可能だがセキュリティクリティカル |
| R1: Bearer 認証 middleware | S | Low | 標準パターン |
| R2: グループ管理 | M | Medium | GitHub Webhook 登録失敗時のロールバック（R2-8）が複雑 |
| R3: 通知発行 + FCM | M | High | FCM v1 JWT 生成（RN-2）が未解決 |
| R3: ステータス算出 | S | Low | 時刻比較ロジック、D1 JOIN クエリのみ |
| R4: 投稿 + GitHub コミット取得 | M | Medium | "直近のコミット"の定義（RN-3）が曖昧 |
| R4: ぼかしフィード | S | Low | サーバー側レスポンス加工、テスト容易 |
| R5: Webhook 受信 | S | Low | HMAC 検証は標準ライブラリ、冪等 INSERT は UNIQUE 制約で対応済み |
| R6: FCM トークン管理 | S | Low | 単純 UPSERT |
| R7: セキュリティ共通 | S | Low | 各ハンドラーで一貫した JSON エラー形式を出力する共通 helper で対応 |
| **全体** | **L** | **High** | **D1 アクセス方式（RN-1）が未確定のため全 repository 層がブロック状態** |

---

## 5. スキーマ追加マイグレーション

以下の `0002_add_groups_name.sql` が必要：

```sql
ALTER TABLE groups ADD COLUMN name TEXT NOT NULL DEFAULT '';
```

---

## 6. 次フェーズへの推奨事項

### 設計フェーズで確定すべき決定事項

1. **RN-1 最優先**: D1 HTTP API vs Workers プロキシを調査・選択し、repository 層のインターフェース設計の前提を固める
2. **RN-2**: FCM v1 JWT の Go 実装方針（`golang.org/x/oauth2/google` 採用可否）
3. **RN-3**: GitHub "最近のコミット" の取得 API エンドポイントと時間窓の定義
4. **HTTP ルーター選定**: `net/http` + 手動パース vs `chi` / `gorilla/mux` — Workers Container では外部依存は問題ないが、バイナリサイズとコールドスタートへの影響を考慮

### 設計フェーズでのアウトプット

- `internal/repository` の DB アクセス抽象化インターフェース（D1 アクセス方式確定後）
- `pkg/github` / `pkg/fcm` のクライアント API 設計
- エンドポイント一覧と各 handler の入出力 JSON スキーマ

---

_分析完了: `/kiro:spec-design begit-backend-api` で設計フェーズへ進んでください_
