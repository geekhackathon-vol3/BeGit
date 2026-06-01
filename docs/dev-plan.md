# BeGit; 5日間開発フロー

> 作成日: 2026-05-31  
> ハッカソン: 技育CAMP ハッカソン2026 vol.3

---

## 現状スナップショット

### インフラ（cloudflare-infra spec）

| タスク | 状態 | 備考 |
|--------|------|------|
| Terraform IaC（D1・R2） | ✅ 完了 | |
| wrangler.toml & package.json | ✅ 完了 | wrangler.toml に未コミット変更あり |
| Workers TypeScript エントリーポイント | ✅ 完了 | |
| Go API Dockerfile | ✅ 完了 | |
| D1 マイグレーション SQL | ⚠️ ファイルは存在するが tasks.md 未チェック | `0001_initial.sql` 実体あり |
| Makefile（deploy/secrets-init/warmup） | ✅ 完了 | |
| インフラ動作確認（ローカル + デプロイ後） | ❌ 未実施 | |

### Go バックエンド

`backend/cmd/server/main.go` のみ存在（全リクエストに `"BeGit API"` を返すだけ）。  
**ビジネスロジック・ハンドラー・サービス・リポジトリ層ゼロ。最大のボトルネック。**

D1 スキーマも spec.md のデータモデルより不足（Notification・Photo・Reaction・Comment・GroupRepository テーブルが未定義）。

### iOS アプリ

- 実装済み画面: Login / RepositoryList（グループ一覧） / RepositoryDashboard / AddRepository / MakeNotification / NotificationResult
- **全て MockData / MockAuthAPI 接続** — バックエンドと未接続
- 未実装画面: **フィード（ぼかし解放）/ 投稿作成（GitHub 情報自動取得）/ リアクション / コメント**

---

## MVP コアフロー

```
GitHub OAuth ログイン
  → グループ作成（リポジトリ指定 + コラボレーター自動参加）
    → 通知発行（1スプリント1人1回）
      → 投稿作成（GitHub 情報自動取得 + 写真）
        → フィード表示（未投稿はぼかし → 投稿後に解放）
          → リアクション
            → Push 通知（commit/PR review → FCM → APNs）
```

---

## 5日間フロー

### Day 1（5/31・今日）— インフラ完成 & Go 骨格

**目標:** デプロイできる状態にする + Go のレイヤードアーキテクチャを構築

#### インフラ

- [ ] `tasks.md` の Task 3.3 を完了マーク
- [ ] `backend/wrangler.toml` の変更をコミット
- [ ] `wrangler dev` でローカル Workers 起動確認（Task 5.1）
- [ ] `make terraform-apply` → `make deploy` で Cloudflare へ初回デプロイ（Task 5.2）
- [ ] `make secrets-init` の手順に従いシークレット4件を登録

#### Go バックエンド — 骨格

- [ ] ディレクトリ構造を作成

```
backend/
  cmd/server/main.go          # ← HTTP サーバー起動・ルーティング
  internal/
    handler/                  # HTTP ハンドラー（リクエスト/レスポンス整形）
    service/                  # ビジネスロジック
    repository/               # D1 アクセス
  pkg/
    github/                   # GitHub API クライアント
    fcm/                      # FCM HTTP API クライアント
    crypto/                   # トークン暗号化（DB_ENCRYPTION_KEY）
```

- [ ] `0001_initial.sql` を iOS モデル対応スキーマに書き直し済み（詳細は下記「DB スキーマと iOS モデルの対応」参照）

- [ ] `POST /auth/github` 実装（GitHub OAuth code → access token 交換 → users テーブル upsert → JWT セッショントークン返却）

---

### Day 2（6/1）— Go バックエンド: グループ & 通知 API

**目標:** グループ作成 → 通知発行の垂直スライスを完成させる

#### API 実装

- [ ] `pkg/github/` — GitHub API クライアント
  - `GET /user` — ユーザー情報取得
  - `GET /repos/{owner}/{repo}/collaborators` — コラボレーター一覧
  - `GET /users/{username}` — ユーザー検索
  - `GET /repos/{username}/repos` + commits — 最近のリポジトリ & コミット

- [ ] `POST /groups` — グループ作成
  - `groups` + `group_repositories` に INSERT
  - GitHub API でコラボレーター一覧を取得
  - BeGit; 連携済みユーザー（users テーブルに存在）を `group_members` に自動 INSERT

- [ ] `GET /groups/:id` — グループ情報 + メンバー一覧取得

- [ ] `POST /groups/:id/sync-members` — コラボレーター再同期

- [ ] `DELETE /groups/:id/members/me` — グループ脱退

- [ ] `POST /devices` — FCM token 登録（`fcm_tokens` upsert）

- [ ] `DELETE /devices/:token` — デバイストークン削除

- [ ] `POST /groups/:id/notifications` — 通知発行
  - 1スプリント1人1回チェック（`notifications` テーブルで検証）
  - `notifications` テーブルに INSERT
  - グループメンバー全員の FCM token を取得 → Day 3 以降で FCM 送信

#### iOS（並行作業）

- [ ] `BackendAuthAPI` を実装（MockAuthAPI を本物の HTTP 呼び出しに差し替え）
- [ ] `APIClient` 基盤クラス作成（ベース URL・認証ヘッダー・エラーハンドリング）

---

### Day 3（6/2）— Go バックエンド: 投稿 & フィード

**目標:** 投稿作成 → フィード表示（ぼかし制御）の垂直スライスを完成させる

#### API 実装

- [ ] `GET /github/repos` — 最近のリポジトリ一覧（投稿画面のサジェスト用）
- [ ] `GET /github/commits?repo={repo}&branch={branch}` — コミット情報（件数・diff・最新メッセージ）

- [ ] `POST /posts` — 投稿作成
  - `posts` テーブルに INSERT
  - `photos` テーブルに INSERT（R2 presigned URL または Worker R2 binding で直接アップロード）
  - `notification_id` から経過時間を計算し `status`（on_time / late）を自動セット

- [ ] `GET /groups/:id/feed` — フィード取得（**ぼかし制御がコア**）
  - リクエストユーザーが通知後未投稿の場合: `photo_key` / `memo` / `latest_commit_message` を null にして返す
  - 投稿済みの場合: 全フィールドを返す
  - 通知がない通常時: 全て返す

- [ ] `GET /posts/:id` — 投稿詳細

- [ ] `POST /posts/:id/reactions` — リアクション追加/削除（トグル）

#### iOS（並行作業）

- [ ] `RepositoryListViewModel` のモックを実 API に接続（グループ一覧）
- [ ] `AddRepositoryViewModel` → `POST /groups` に接続
- [ ] `MakeNotificationViewModel` → `POST /groups/:id/notifications` に接続

---

### Day 4（6/3）— Go: Webhook & FCM + iOS: フィード & 投稿作成画面

**目標:** Push 通知フロー完成 + iOS フィード画面実装

#### Go バックエンド

- [ ] `pkg/fcm/` — FCM HTTP API クライアント
  - `FIREBASE_SERVICE_ACCOUNT_JSON` から JWT アクセストークン生成
  - `POST https://fcm.googleapis.com/v1/projects/{project}/messages:send` 呼び出し

- [ ] `POST /github/webhook` — GitHub Webhook ハンドラー
  - `X-Hub-Signature-256` HMAC-SHA256 検証（失敗 → 403）
  - `push` event: pusher の github_login → users テーブルで検索 → FCM 通知送信
  - `pull_request_review` event: 同様に通知送信
  - 通知ペイロード: `type: "commit_detected"` / `"pr_review_detected"`

- [ ] 通知発行時（`POST /groups/:id/notifications`）の FCM 送信を実装
  - グループメンバー全員に「今なに作ってる？」通知

- [ ] `POST /posts/:id/comments` — コメント追加
- [ ] `GET /posts/:id/comments` — コメント一覧

#### iOS

- [ ] **FeedView** 実装
  - グループフィードを API から取得
  - 未投稿時: カード全体をぼかし（`blur(radius: 10)` + overlay）
  - 投稿済み: 通常表示
  - Pull-to-refresh
  - `On Time` / `Late` / `Missed` バッジ

- [ ] **PostCreationView** 実装
  - GitHub リポジトリサジェスト（`GET /github/repos`）
  - コミット情報自動取得（`GET /github/commits`）
  - 写真添付（カメラ / フォトライブラリ）
  - `POST /posts` 送信

#### AppDelegate 設定

- [ ] Firebase SDK を iOS プロジェクトに追加（`FirebaseMessaging`）
- [ ] APNs デバイストークン → FCM token 変換 → `POST /devices` 登録
- [ ] `didReceiveRemoteNotification`: `type == "commit_detected"` → PostCreationView へ遷移

---

### Day 5（6/4）— 統合 & デモ準備

**目標:** E2E 動作確認 + Cloudflare 本番デプロイ + デモ磨き

#### iOS 仕上げ

- [ ] `RepositoryDashboardView` にフィードタブを追加（FeedView を組み込む）
- [ ] リアクション UI（emoji ボタン → `POST /posts/:id/reactions`）
- [ ] コメント UI（コメント一覧 + 入力欄）
- [ ] プライバシー設定 UI（投稿時に privacy_level を選択）
- [ ] エラーハンドリング & ローディング状態の統一

#### デプロイ & 検証

- [ ] `git commit` — 全変更をまとめてコミット
- [ ] `make terraform-apply` — D1 ID を wrangler.toml に反映（変更ある場合）
- [ ] `wrangler d1 migrations apply begit-db` — スキーマ更新を本番 D1 に適用
- [ ] `make deploy` — Docker build → wrangler deploy → migration
- [ ] `wrangler secret list` — 4シークレット確認
- [ ] `curl https://begit.workers.dev/` — Workers 疎通確認
- [ ] GitHub Webhook を Workers URL に設定（GitHub リポジトリ設定画面）
- [ ] iOS Simulator から E2E テスト（ログイン → グループ → 通知 → 投稿 → フィード解放）

#### デモ準備

- [ ] `make warmup` — コンテナウォームアップ（コールドスタート対策）
- [ ] テストアカウント 2 名分でシナリオ通し確認
- [ ] デモシナリオ: 通知発行 → 全員投稿 → `On Time` バッジ取得の流れ

---

## 優先順位マップ

```
P0（デモで絶対必要）
  ├── GitHub OAuth ログイン（完成度: iOS 80%, Go 0%）
  ├── グループ作成・参加（iOS 70%, Go 0%）
  ├── 通知発行（iOS 70%, Go 0%）
  ├── 投稿作成 + GitHub 情報自動取得（iOS 20%, Go 0%）
  └── フィード + ぼかし解放（iOS 0%, Go 0%）

P1（デモでできると強い）
  ├── Push 通知 commit → iPhone（Go 0%, iOS 30%）
  ├── リアクション（iOS 0%, Go 0%）
  └── On Time / Late バッジ（Go ロジック 0%, iOS 0%）

P2（余力があれば）
  ├── コメント
  └── プライバシー設定
```

---

## リスクと対策

| リスク | 影響 | 対策 |
|--------|------|------|
| Go バックエンドの実装量が多い | 高 | Day 2-3 を Go に全集中、iOS は並行で Day 4 以降に接続 |
| Workers Container コールドスタート（デモ中に沈黙） | 高 | `make warmup` でデモ直前にウォームアップ |
| Firebase SDK セットアップに時間がかかる | 中 | Day 4 に専用時間を確保。GoogleService-Info.plist の準備を事前に |
| D1 スキーマ変更で既存データが壊れる | 中 | Day 1 の ADD COLUMN でデータを保持。DROP は避ける |
| iOS ↔ Go の API 型不一致 | 中 | Day 2 のうちに JSON レスポンス形式を先に決めてモックを更新 |
| GitHub OAuth コールバック URL の設定漏れ | 低 | GitHub OAuth App の Callback URL に Workers URL を追加 |

---

## DB スキーマと iOS モデルの対応

### テーブル一覧（統合済み）

| テーブル | 対応 iOS モデル | 備考 |
|---------|---------------|------|
| `users` | `GitHubUser`, `RepositoryMember` | `avatar_url` 追加（旧スキーマになかった） |
| `groups` | `Repository` | `repo_full_name` = `Repository.name` |
| `group_members` | `Repository.members` | `role`, `auto_joined` 追加 |
| `notifications` | `RepositoryNotification` | `UNIQUE(group_id, sent_by, sprint_index)` で1スプリント1人1回を保証 |
| `posts` | `RepositoryActivity` + 将来の `FeedPost` | Dashboard と Feed を **統一**。`post_type` + `body` で区別 |
| `photos` | `FeedPost.photos` | R2 キーを保存 |
| `reactions` | `RepositoryReaction` + `FeedPost.reactions` | 1テーブルに統合、iOS が `post_type` ごとに絵文字セット切り替え |
| `comments` | `FeedPost.comments` | |
| `fcm_tokens` | Push 通知用 | |

### フィールドマッピング

```
iOS Repository              → DB groups
  .id (UUID)                → .id (INTEGER) ← API が Int を返す
  .name ("owner/repo")      → .repo_full_name (TEXT)
  .memberCount              → COUNT(group_members) で算出
  .members [RepositoryMember]→ JOIN users via group_members

iOS RepositoryMember        → DB users
  .login                    → .github_login
  .avatarURL                → .avatar_url   ← 旧スキーマに存在しなかった

iOS RepositoryActivity      → DB posts
  .type (.commit/.pullRequest/.comment) → .post_type ('commit'|'pull_request'|'comment')
  .title                    → .latest_commit_message or body の先頭
  .comment                  → .body         ← 投稿文 "認証入れたよ〜"
  .date                     → .created_at
  .imageName (SF Symbol)    → DB 不要（iOS 側で post_type → アイコン名を決定）
  .author                   → JOIN users

iOS RepositoryReaction      → DB reactions
  .heart/.check             → .reaction_type ('heart'|'check')
  ※ post_type に応じて iOS 側で絵文字セットを切り替える

iOS RepositoryNotification  → DB notifications
  .id (UUID)                → .id (INTEGER)
  .comment                  → .message
  .createdAt                → .sent_at
  .selectedMembers          → API で group_members 全員に FCM 送信（DB には保存しない）
```

### `posts.post_type` の値一覧

| post_type | 意味 | 絵文字セット（iOS） | SF Symbol（iOS） |
|-----------|------|------------------|-----------------|
| `commit` | コミット報告 | lgtm / watching / grass | `chevron.left.forwardslash.chevron.right` |
| `pull_request` | PR オープン / レビュー | lgtm / review / merge | `arrow.triangle.pull` |
| `issue` | Issue 対応 | lgtm / watching | `exclamationmark.circle` |
| `review` | コードレビュー完了 | lgtm / review | `eye` |
| `comment` | 進捗メッセージ（作業はできないが近況を共有） | heart / check | `text.bubble` |

### API レスポンス形式

```json
// POST /auth/github → AuthResponse
// iOS: AuthResponse { accessToken, githubUser }
{
  "session_token": "jwt...",
  "user": { "id": 1, "github_login": "octocat", "avatar_url": "https://..." }
}

// GET /groups → [Repository]
{
  "groups": [
    {
      "id": 1,
      "repo_full_name": "apple/swift",
      "member_count": 4,
      "members": [
        { "github_login": "octocat", "avatar_url": "https://..." }
      ]
    }
  ]
}

// GET /groups/:id/activities → [RepositoryActivity]
{
  "activities": [
    {
      "id": 1,
      "type": "commit",
      "title": "認証つけたよー",
      "body": "ようやく動いた",
      "created_at": "2026-05-31T12:00:00Z",
      "author": { "github_login": "octocat", "avatar_url": "..." },
      "reaction": { "type": "check", "reacted_by_me": true }
    }
  ]
}

// POST /groups/:id/notifications → RepositoryNotification
{
  "id": 1,
  "message": "今なに作ってる？",
  "sent_by": { "github_login": "octocat", "avatar_url": "..." },
  "sent_at": "2026-05-31T12:00:00Z"
}

// GET /groups/:id/feed → [FeedPost] ← iOS 未実装、将来用
{
  "posts": [
    {
      "id": 1,
      "author": { "github_login": "octocat", "avatar_url": "..." },
      "status": "on_time",
      "blurred": false,
      "repo_full_name": "user/repo",
      "branch_name": "main",
      "commit_count": 5,
      "additions": 120,
      "deletions": 30,
      "latest_commit_message": "認証つけたよー",
      "memo": "ようやく動いた",
      "tags": ["Go", "Swift"],
      "photos": [{ "url": "https://workers.url/photos/xxx", "type": "code_screenshot" }],
      "reactions": [{ "type": "lgtm", "count": 2, "reacted_by_me": true }],
      "created_at": "2026-05-31T12:00:00Z"
    }
  ]
}
```

---

*BeGit; — あなたの開発を、チームのワクワクに。*
