# Requirements Document

## Introduction

BeGit は「BeReal × GitHub」コンセプトの開発者向けチーム SNS アプリのバックエンド REST API。iOS クライアント（SwiftUI）からのリクエストを受け付け、GitHub OAuth 2.0 認証・グループ管理・通知発行・投稿フィード・Webhook 受信などの機能を提供する。Cloudflare Workers をエントリーポイントとし、Workers Containers 上で動作する Go サーバーが実際のビジネスロジックを処理する。

## Boundary Context

- **In scope**: Go バックエンド REST API の全エンドポイント実装、D1 データアクセス、R2 画像ストレージ連携、FCM Push 通知送信、GitHub API 連携、Webhook 受信処理
- **Out of scope**: iOS クライアント実装、Cloudflare Workers のルーティング設定（Terraform/Wrangler IaC）、Firebase SDK の iOS 側設定
- **Adjacent expectations**: GitHub OAuth App の事前設定、FCM プロジェクトのセットアップ、D1 マイグレーション済みスキーマ（`0001_initial.sql`）の存在

---

## Requirements

### Requirement 1: GitHub OAuth 認証

**Objective:** As a iOS ユーザー, I want GitHub OAuth でサインイン・サインアップできる, so that BeGit 固有のアカウント管理なしに GitHub アカウントで即利用開始できる

#### Acceptance Criteria

1. When iOS クライアントが GitHub authorization code を含む `POST /auth/github` リクエストを送信した, the BeGit API サーバー shall `GITHUB_CLIENT_SECRET` を使って GitHub の OAuth トークンエンドポイント（`https://github.com/login/oauth/access_token`）へ code を送信し access_token を取得する
2. When access_token の取得に成功した, the BeGit API サーバー shall GitHub Users API（`GET /user`）を呼び、ユーザー情報（`login`, `id`, `avatar_url`, `name`）を取得する
3. When GitHub ユーザーが D1 の `users` テーブルに存在しない, the BeGit API サーバー shall 新規ユーザーレコードを挿入し、access_token を `DB_ENCRYPTION_KEY` で暗号化して保存する
4. When GitHub ユーザーが D1 の `users` テーブルに既に存在する, the BeGit API サーバー shall 既存レコードの `encrypted_access_token` を最新の暗号化済みトークンで上書き更新する
5. When 認証フローが正常完了した, the BeGit API サーバー shall ユーザー情報と Bearer トークン（access_token 平文）を JSON レスポンスとして iOS クライアントに返す
6. If GitHub が無効または期限切れのコードを返した場合, the BeGit API サーバー shall 401 Unauthorized とエラー詳細を JSON で返す
7. The BeGit API サーバー shall `/auth/github` および `/webhook/github` 以外の全エンドポイントに対して `Authorization: Bearer <token>` ヘッダーを必須とし、不正または欠損の場合は 401 Unauthorized を返す

---

### Requirement 2: グループ管理

**Objective:** As a BeGit ユーザー, I want GitHub リポジトリをベースにグループを作成・参照できる, so that チームメンバーと同じリポジトリの開発状況をグループ単位で共有できる

#### Acceptance Criteria

1. When 認証済みユーザーが `GET /groups` を呼んだ, the BeGit API サーバー shall そのユーザーが `group_members` テーブルに所属する全グループの一覧（`id`, `name`, `repo_full_name`, `avatar_url`）を返す
2. When 認証済みユーザーが `POST /groups` に `repo_full_name`・`name` を含むリクエストを送信した, the BeGit API サーバー shall D1 の `groups` テーブルにグループを作成し、作成者を `group_members` に `owner` ロールで追加する
3. When グループが作成された, the BeGit API サーバー shall GitHub の `POST /repos/{owner}/{repo}/hooks` を呼び、`push` イベントと `pull_request_review` イベントを受信する Webhook を `/webhook/github` に登録する
4. When グループが作成された, the BeGit API サーバー shall `GET /repos/{owner}/{repo}/collaborators` を呼び、BeGit の `users` テーブルに登録済みのコラボレーターを `group_members` に `member` ロールで自動追加する
5. When 認証済みユーザーが `GET /groups/:id` を呼んだ, the BeGit API サーバー shall グループ詳細情報とメンバー一覧（`user_id`, `login`, `avatar_url`, `role`）を返す
6. If 認証済みユーザーが `group_members` に所属していないグループへアクセスした場合, the BeGit API サーバー shall 403 Forbidden を返す
7. If 指定された `:id` に対応するグループが存在しない場合, the BeGit API サーバー shall 404 Not Found を返す
8. If `POST /groups` 時に GitHub Webhook 登録に失敗した場合, the BeGit API サーバー shall グループ作成をロールバックし 502 Bad Gateway を返す

---

### Requirement 3: 通知発行（BeGit Time）

**Objective:** As a グループメンバー, I want BeGit Time 通知を発行・確認できる, so that チームの投稿チャレンジを起動し結果を追跡できる

#### Acceptance Criteria

1. When 認証済みグループメンバーが `POST /groups/:id/notifications` を呼んだ, the BeGit API サーバー shall `sprints` テーブルの当日スプリントを取得または作成し、`notifications` テーブルに通知レコードを挿入する
2. When 通知レコードの挿入に成功した, the BeGit API サーバー shall FCM HTTP API を経由してグループの全メンバーの `fcm_tokens` に対して Push 通知を送信する
3. The BeGit API サーバー shall 1スプリントあたり1ユーザー1通知のみ許可し、`UNIQUE(sprint_id, sent_by)` 制約に違反する場合は 409 Conflict を返す
4. When 認証済みグループメンバーが `GET /groups/:id/notifications/:nid` を呼んだ, the BeGit API サーバー shall 通知発行後の各メンバーの投稿ステータス（`On Time` / `Late` / `Missed`）を算出して返す
5. When 通知発行時刻から1時間以内に投稿が存在する場合, the BeGit API サーバー shall そのメンバーのステータスを `On Time` と判定する
6. When 通知発行時刻から1時間超に投稿が存在する場合, the BeGit API サーバー shall そのメンバーのステータスを `Late` と判定する
7. While 通知発行後に投稿が存在しない場合, the BeGit API サーバー shall そのメンバーのステータスを `Missed` と判定する
8. If 指定された `:nid` に対応する通知が存在しない場合, the BeGit API サーバー shall 404 Not Found を返す

---

### Requirement 4: 投稿とフィード

**Objective:** As a BeGit ユーザー, I want 自分の開発活動を投稿し、チームメンバーのフィードを閲覧できる, so that チームの現在の開発状況をゲーム感覚でリアルタイムに把握できる

#### Acceptance Criteria

1. When 認証済みグループメンバーが `POST /groups/:id/posts` にリクエストを送信した, the BeGit API サーバー shall GitHub API からそのユーザーの直近のコミット情報（コミット数、変更行数、最新コミットメッセージ、`repo_full_name`）を自動取得する
2. When GitHub コミット情報の取得に成功した, the BeGit API サーバー shall `posts` テーブルにレコードを挿入する（`body`, `repo_full_name`, `latest_commit_message`, `commit_count`, `additions`, `deletions` を含む）
3. When 認証済みグループメンバーが `GET /groups/:id/posts` を呼んだ, the BeGit API サーバー shall グループのフィード一覧を投稿日時の降順で返す
4. While リクエストユーザーが現在のスプリントで未投稿の場合, the BeGit API サーバー shall 他メンバーの posts レスポンスの `body` / `repo_full_name` / `latest_commit_message` フィールドを `null` にして返す（ぼかし制御）
5. When リクエストユーザーが現在のスプリントで投稿済みの場合, the BeGit API サーバー shall 全メンバーのフィードを全フィールド開示で返す
6. If `POST /groups/:id/posts` 時に GitHub API の呼び出しに失敗した場合, the BeGit API サーバー shall 502 Bad Gateway を返し投稿レコードを保存しない
7. If リクエストユーザーが対象グループの `group_members` に所属していない場合, the BeGit API サーバー shall 403 Forbidden を返す

---

### Requirement 5: GitHub Webhook 受信

**Objective:** As a システム, I want GitHub からの push / pull_request_review イベントを安全かつ冪等に受信できる, so that リアルタイムの開発活動をシステム側で検知し将来の拡張に対応できる

#### Acceptance Criteria

1. When `POST /webhook/github` リクエストが到着した, the BeGit API サーバー shall `X-Hub-Signature-256` ヘッダーを HMAC-SHA256 でペイロードに対して計算・比較し、署名が一致する場合のみ処理を継続する
2. If `X-Hub-Signature-256` の検証に失敗した場合, the BeGit API サーバー shall 403 Forbidden を返しイベントを処理しない
3. When 署名検証に成功した, the BeGit API サーバー shall `X-GitHub-Delivery` ヘッダーの値を `github_webhook_deliveries` テーブルに INSERT し、一意制約違反（重複）の場合は処理をスキップして 200 OK を即座に返す
4. When `push` イベントを受信した, the BeGit API サーバー shall リポジトリ名から対応するグループを特定し、スプリント情報を必要に応じて更新する
5. When `pull_request_review` イベントを受信した, the BeGit API サーバー shall リポジトリ名から対応するグループを特定し、スプリント情報を必要に応じて更新する
6. The BeGit API サーバー shall `/webhook/github` エンドポイントに Bearer 認証を要求しない（GitHub からの受信のため署名検証で代替する）
7. If 対応するグループが見つからない場合, the BeGit API サーバー shall 200 OK を返してイベントを無視する（エラーにしない）

---

### Requirement 6: FCM トークン管理

**Objective:** As a iOS ユーザー, I want アプリ起動時に FCM トークンを登録・更新できる, so that 最新の Push 通知トークンが常にサーバーに保持され通知が確実に届く

#### Acceptance Criteria

1. When 認証済みユーザーが `PUT /me/fcm-token` に `fcm_token` フィールドを含むリクエストを送信した, the BeGit API サーバー shall `fcm_tokens` テーブルにそのユーザーの FCM トークンを INSERT OR REPLACE（UPSERT）する
2. When 同一ユーザーが既存と異なる FCM トークンで `PUT /me/fcm-token` を呼んだ, the BeGit API サーバー shall 既存レコードのトークン値を新しい値に更新し重複登録を防ぐ
3. When FCM トークンの登録・更新に成功した, the BeGit API サーバー shall 200 OK を返す
4. If リクエストボディに `fcm_token` フィールドが存在しないまたは空文字の場合, the BeGit API サーバー shall 400 Bad Request と詳細エラーを返す

---

### Requirement 7: セキュリティと共通仕様

**Objective:** As a システム管理者, I want API が一貫したセキュリティポリシーとエラーハンドリングを持つ, so that 不正アクセスやデータ漏洩リスクを最小化できる

#### Acceptance Criteria

1. The BeGit API サーバー shall GitHub `access_token` を `DB_ENCRYPTION_KEY` による AES-GCM 暗号化を施して D1 に保存し、平文では保存しない
2. The BeGit API サーバー shall `GITHUB_CLIENT_SECRET` / `DB_ENCRYPTION_KEY` / FCM サービスアカウント情報を Cloudflare Workers Secrets または環境変数から取得し、ソースコードにハードコードしない
3. The BeGit API サーバー shall 全レスポンスを JSON 形式で返し、`Content-Type: application/json` ヘッダーを付与する
4. If 内部エラー（DB エラー・外部 API タイムアウトなど）が発生した場合, the BeGit API サーバー shall 500 Internal Server Error と `{ "error": "<message>" }` 形式の JSON を返す（スタックトレース・内部詳細は含めない）
5. When バリデーションエラーが発生した場合, the BeGit API サーバー shall 422 Unprocessable Entity と `{ "error": "<field>: <reason>" }` 形式のエラーメッセージを返す
6. The BeGit API サーバー shall handler → service → repository の依存方向を厳守し、逆方向の依存を持たない
7. The BeGit API サーバー shall `cmd/server/` をエントリーポイント、`internal/handler/`・`internal/service/`・`internal/repository/` をビジネスロジック層、`pkg/github/`・`pkg/fcm/` を外部連携層として構成する
