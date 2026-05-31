# BeGit 仕様書

**バージョン:** 1.0.0  
**作成日:** 2026-05-28  
**ステータス:** Draft

---

## 目次

1. [プロダクト概要](#1-プロダクト概要)
2. [ターゲットユーザー](#2-ターゲットユーザー)
3. [コンセプト・世界観](#3-コンセプト世界観)
4. [機能一覧](#4-機能一覧)
5. [機能詳細仕様](#5-機能詳細仕様)
   - 5.1 認証（GitHub OAuth）
   - 5.2 グループ・通知機能
   - 5.3 投稿作成画面
   - 5.4 フィード・リアクション
   - 5.5 リポジトリ連携
   - 5.6 プライバシー設定
6. [データモデル（概要）](#6-データモデル概要)
7. [API設計（概要）](#7-api設計概要)
8. [技術スタック](#8-技術スタック)
9. [非機能要件](#9-非機能要件)
10. [MVP スコープ](#10-mvp-スコープ)
11. [将来拡張](#11-将来拡張)

---

## 1. プロダクト概要

| 項目 | 内容 |
|------|------|
| プロダクト名 | BeGit |
| ジャンル | 開発者向け チーム開発支援 SNS アプリ（iOS） |
| コンセプト | BeReal × GitHub — オンライン＆オフラインのハイブリッド開発を盛り上げる |
| 一言説明 | 一定期間内にメンバーが任意タイミングで「通知」を発行し、全員が1時間以内に今の開発状況をコミットまたは投稿するアプリ |

### 背景・課題

- フルリモート・分散チームでは「今何作ってるか」が見えにくい
- Slackのスタンドアップは義務感が強く、継続しにくい
- 物理的に離れたメンバー（例：九州在住）でも「一緒に作ってる感」を出したい

---

## 2. ターゲットユーザー

- チーム開発を楽しみたい開発者
- アジャイル・スクラムスタイルで開発しているチーム
- GitHubをバージョン管理に使っているエンジニア
- とにかくエンジョイ重視の開発者（義務感ではなくゲーム感覚で参加したい人）

---

## 3. コンセプト・世界観

### BeReal との違い

| BeReal | BeGit |
|--------|-------|
| ランダム通知 | **メンバーが任意タイミングで通知を発行** |
| 写真（表・裏カメラ）| コードスクリーンショット + 作業環境写真 |
| 日常の瞬間をシェア | 開発の瞬間をシェア |
| 個人向け | チーム開発向け |

### ゲーミフィケーション要素

- 1スプリントに1人1回しか通知を発行できない → **「いつ打つか」に戦略性が生まれる**
- 通知後1時間以内の投稿は **On Time** バッジ付き
- 遅れると **Late**、無視すると **Missed**

---

## 4. 機能一覧

| # | 機能 | 優先度 | MVP |
|---|------|--------|-----|
| 1 | GitHub OAuth 認証 | 必須 | ✅ |
| 2 | グループ作成・招待 | 必須 | ✅ |
| 3 | 通知発行（1スプリント1人1回） | 必須 | ✅ |
| 4 | 投稿作成（GitHub情報自動取得） | 必須 | ✅ |
| 5 | フィード表示（投稿後に詳細解放） | 必須 | ✅ |
| 6 | リアクション | 必須 | ✅ |
| 7 | コメント | 高 | ✅ |
| 8 | プライバシー設定 | 高 | ✅ |
| 9 | 写真添付（コード / 机 / 環境） | 高 | ✅ |
| 10 | PR / Issue 連携 | 中 | ❌（将来） |

---

## 5. 機能詳細仕様

### 5.1 認証（GitHub OAuth）

**概要:** GitHub OAuthを用いてログインする。GitHubアカウントがそのままBeGitのアカウントになる。

**必要スコープ:**

| スコープ | 用途 | 必須 |
|--------|------|------|
| `read:user` | ユーザープロフィール取得 | 必須 |
| `public_repo` | 公開リポジトリのcommit/PR/issue取得 | 必須 |
| `repo` | プライベートリポジトリへのアクセス | 任意（ユーザー選択） |

**フロー:**

```
1. アプリ起動
2. 「GitHubでログイン」ボタンをタップ
3. Safari / ASWebAuthenticationSession でOAuth認証
4. コールバックでアクセストークンを受け取る
5. バックエンドにトークンを送信・ユーザー作成/更新
6. ホーム画面へ遷移
```

---

### 5.2 グループ・通知機能

#### グループ

| 項目 | 仕様 |
|------|------|
| 作成 | ユーザーが任意でグループを作成し、GitHubリポジトリを紐付ける |
| 自動参加 | **紐付けられたリポジトリにコラボレーターとして登録されており、かつBeGit連携済みのユーザーが自動的にグループメンバーになる** |
| 参加チェックタイミング | グループ作成時 + 新規ユーザーがBeGit連携時 + 定期同期（TBD） |
| スプリント期間 | グループ作成時に設定（例：1週間） |
| 人数上限 | TBD |

**自動参加フロー:**

```
1. グループオーナーがグループ作成 & GitHubリポジトリを指定
2. BeGitサーバーがGitHub APIで対象repoのコラボレーター一覧を取得
   GET /repos/{owner}/{repo}/collaborators
3. 一覧の中でBeGit連携済みユーザーを抽出
4. 該当ユーザーをGroupMemberに自動追加
5. 追加されたユーザーにFCM通知「{repo名} のグループに追加されました 🎉」
```

> **注:** ユーザーは参加を辞退（脱退）することも可能。

#### 通知発行ルール

| 項目 | 仕様 |
|------|------|
| 発行権 | **1スプリントにつき1人1回のみ** |
| タイミング | 発行者が任意のタイミングで発行（ランダムではない） |
| 意図 | 「今みんな作業してそうな時間」を見計らって打つ → ゲーミフィケーション |
| 送信先 | グループ全員 |
| 通知文例 | `「今なに作ってる？」` `「BeGit Time ⏰」` `「1時間以内に開発状況を投稿してください」` |

#### 投稿期限ステータス

| ステータス | 条件 |
|----------|------|
| `On Time` | 通知から1時間以内に投稿 |
| `Late` | 1時間超過後に投稿 |
| `Missed` | 投稿なし（スプリント終了後に自動付与） |

#### commit / PR検知 → スマホ通知の技術構成

**概要:** GitHubでcommitやPR reviewが発生したことをサーバーが検知し、該当ユーザーのスマホへFCM経由でpush通知を送る。FCMがAPNsへの配信を中継するため、バックエンドはAPNsを直接管理しない。

**全体フロー:**

```
[開発者がgit push / PR reviewを実行]
        ↓
[GitHub Webhook]
   POST /github/webhook  ← BeGitサーバーのエンドポイント
        ↓
[BeGitサーバー (Go)]
  1. X-Hub-Signature-256 でリクエストを検証（秘密鍵で署名確認）
  2. eventの種別を判定
     - push event       → commitを検知
     - pull_request_review event → PR reviewを検知
  3. 対象ユーザーのFCM registration tokenをDBから取得
  4. FCM HTTP API へ通知リクエストを送信
        ↓
[FCM (Firebase Cloud Messaging)]
        ↓
[APNs (Apple Push Notification service)]
        ↓
[ユーザーのiPhone に通知到達 🔔]
```

**GitHub Webhookの設定:**

| 項目 | 値 |
|------|---|
| Payload URL | `https://<BeGitサーバー>/github/webhook` |
| Content type | `application/json` |
| Secret | 任意の秘密文字列（署名検証に使用） |
| 購読イベント | `push`, `pull_request_review` |

**Webhookペイロード例（push event）:**

```json
{
  "ref": "refs/heads/main",
  "repository": { "full_name": "user/repo" },
  "pusher": { "name": "githubUsername" },
  "commits": [
    { "id": "abc123", "message": "認証つけたよー", "timestamp": "..." }
  ]
}
```

**FCM通知ペイロード:**

```json
{
  "message": {
    "token": "FCM_REGISTRATION_TOKEN",
    "notification": {
      "title": "commit完了！",
      "body": "認証つけたよー — user/repo main"
    },
    "data": {
      "type": "commit_detected",
      "repo": "user/repo",
      "commit_message": "認証つけたよー"
    },
    "apns": {
      "payload": { "aps": { "sound": "default", "badge": 1 } }
    }
  }
}
```

**iOSアプリ側の実装ポイント:**

```swift
// AppDelegate.swift
func application(_ application: UIApplication,
  didReceiveRemoteNotification userInfo: [AnyHashable: Any]) {
    // type == "commit_detected" なら投稿作成画面へ遷移
    if let type = userInfo["type"] as? String, type == "commit_detected" {
        NavigationRouter.push(.createPost)
    }
}
```

**通知文パターン:**

| トリガー | 通知タイトル | 通知本文 |
|--------|------------|--------|
| push（commit） | `commitを検知しました 📸` | `タップして今の開発状況を投稿しよう` |
| PR review | `PR reviewを検知しました 👀` | `タップして投稿しよう` |

**セキュリティ:**
- Webhook受信時は必ず `X-Hub-Signature-256` ヘッダーをHMAC-SHA256で検証する
- 検証失敗のリクエストは `403` を返して無視する

```go
// Go サーバー側の署名検証（概要）
mac := hmac.New(sha256.New, []byte(webhookSecret))
mac.Write(body)
expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
if !hmac.Equal([]byte(expected), []byte(signature)) {
    http.Error(w, "invalid signature", http.StatusForbidden)
    return
}
```

---

### 5.3 投稿作成画面

#### 自動取得情報（GitHub APIから）

| フィールド | 取得元 | 備考 |
|----------|--------|------|
| Repository名 | GitHub API | 最近commitしたrepoを自動サジェスト |
| Branch名 | GitHub API | 最近pushしたbranchを自動サジェスト |
| 今日のCommit数 | GitHub API | |
| 変更行数（+ / -） | GitHub API diff | |
| 最新Commit Message | GitHub API | デフォルト表示、ユーザーが上書き可 |
| 未Push Commit数 | GitHub API | |
| Open PR | GitHub API | |
| Assigned Issue | GitHub API | |

#### 手動入力フィールド

| フィールド | 種別 | 説明 |
|----------|------|------|
| ひとことメモ | テキスト（任意） | 自由記述。例：「認証つけたよー」 |
| 技術タグ | マルチセレクト | 下記タグ一覧から選択 or 自由入力 |

**技術タグ候補:**  
`Swift` `SwiftUI` `React` `Next.js` `Rails` `Go` `Kotlin` `Flutter` `AI` `AWS` ほか

#### 添付

| 種別 | 説明 |
|------|------|
| コードスクリーンショット | エディタ画面等のスクショ |
| 作業机写真 | BeReal風に「今の自分の環境」を撮影 |
| 作業環境写真 | セットアップ写真等 |

#### 自動サジェスト

- 最近commitしたRepository
- 最近pushしたBranch
- Open PR
- Assigned Issue
- Starred Repository

**サジェスト文例:**
```
「今日は quoline に3 commits しています。これを投稿しますか？」
```

---

### 5.4 フィード・リアクション

#### フィード表示

| 状態 | 表示内容 |
|------|--------|
| 自分が未投稿（通知後） | 他者の投稿内容をぼかして表示。詳細非表示 |
| 自分が投稿済み | 全員の詳細を閲覧可能 |
| 通常時 | 時系列でグループメンバーの投稿一覧 |

#### 投稿カードの構成

**ヘッダー**
- アイコン
- ユーザー名
- 投稿時刻
- `On Time` / `Late` バッジ

**開発情報**
- Repository
- Branch
- Commit数
- Diff（+ / -）
- 最新Commit Message
- 作業メモ

**添付**
- コードスクリーンショット
- 作業環境写真

**技術タグ**

#### リアクション

| emoji | ラベル | 意味 |
|-------|--------|------|
| 👍 | LGTM | いいね |
| 👀 | 見てる | 確認中 |
| 🌱 | 草 | コミット草を称える |
| 💪 | 強い | 進捗が力強い |
| 📝 | レビュー待ち？ | レビューリクエスト |
| 🚀 | Mergeしろ | マージ推奨 |

#### コメント

- テキストコメント
- スレッド形式なし（軽い会話を想定）
- 1コメント最大 TBD 文字

---

### 5.5 リポジトリ連携

ユーザーが投稿時に選択できる項目:

- 最近commitしたrepo
- 最近pushしたbranch
- 最近開いたPR
- Assigned Issue
- Starred Repository

**実装方針:** GitHub REST API v3 / GraphQL API を使用

---

### 5.6 プライバシー設定

#### 隠せる情報

| 項目 | 設定 |
|------|------|
| Private repo名 | 非表示モード |
| Commit Message | 非表示 |
| Diff | 非表示 |
| Organization名/会社名 | マスク |
| スクリーンショット | 自動ぼかし |

#### 投稿範囲

| 範囲 | 説明 | MVP |
|------|------|-----|
| 全体公開 | BeGit全ユーザーに公開 | ✅ |
| フォロワーのみ | フォロワーに限定 | ❌（将来） |
| チームのみ | グループメンバーのみ | ✅ |
| 自分だけ（ログ） | 自分のみ閲覧可（記録用） | ✅ |

---

## 6. データモデル

**DB:** Cloudflare D1（SQLite 互換）  
**ID:** `TEXT` UUID（アプリ側生成）  
**時刻:** `TEXT` ISO8601 UTC

> MVP では 1 Group : 1 GitHub リポジトリ（`groups.repo_full_name` に直接保持）。  
> `privacy_level` の `followers` は将来拡張（[11. 将来拡張](#11-将来拡張)）— MVP では `public | group | private` のみ。

### 6.1 エンティティ一覧

#### User

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `github_id` | INTEGER | NOT NULL, UNIQUE | GitHub ユーザー ID |
| `github_login` | TEXT | NOT NULL, UNIQUE | GitHub login（コラボレーター自動参加のマッチング用） |
| `username` | TEXT | NOT NULL | 表示名 |
| `avatar_url` | TEXT | | アバター URL |
| `access_token_encrypted` | TEXT | NOT NULL | 暗号化 GitHub アクセストークン |
| `token_expires_at` | TEXT | | トークン有効期限 |
| `created_at` | TEXT | NOT NULL | 作成日時 |

#### FCMToken

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `user_id` | TEXT | NOT NULL, FK → users | 所有者 |
| `registration_token` | TEXT | NOT NULL, UNIQUE | FCM デバイストークン |
| `platform` | TEXT | NOT NULL, CHECK `ios` | プラットフォーム |
| `updated_at` | TEXT | NOT NULL | 最終更新 |

#### Group

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `name` | TEXT | NOT NULL | グループ名 |
| `repo_full_name` | TEXT | NOT NULL | 紐付け GitHub リポジトリ（例: `owner/repo`） |
| `sprint_duration_days` | INTEGER | NOT NULL, CHECK > 0 | スプリント期間（日） |
| `created_by` | TEXT | NOT NULL, FK → users | 作成者 |
| `created_at` | TEXT | NOT NULL | 作成日時 |

#### GroupMember

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `group_id` | TEXT | PK, FK → groups | |
| `user_id` | TEXT | PK, FK → users | |
| `role` | TEXT | NOT NULL, CHECK `owner\|member` | 役割 |
| `auto_joined` | INTEGER | NOT NULL, CHECK 0\|1 | リポジトリ連携による自動参加か |
| `joined_at` | TEXT | NOT NULL | 参加日時 |
| `left_at` | TEXT | | NULL = 在籍中（ソフトデリート） |

#### Sprint

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `group_id` | TEXT | NOT NULL, FK → groups | |
| `index_num` | INTEGER | NOT NULL, CHECK ≥ 1, UNIQUE(group_id, index_num) | スプリント番号 |
| `started_at` | TEXT | NOT NULL | 開始日時 |
| `ends_at` | TEXT | NOT NULL | 終了日時 |

#### BeTimeNotification（BeGit Time 通知発行）

> Push 通知（FCM）と区別するため、ドメイン概念は `BeTimeNotification` と呼ぶ。DB テーブル名: `be_time_notifications`。

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `group_id` | TEXT | NOT NULL, FK → groups | |
| `sprint_id` | TEXT | NOT NULL, FK → sprints | |
| `sent_by` | TEXT | NOT NULL, FK → users | 発行者 |
| `message` | TEXT | NOT NULL | 通知メッセージ |
| `sent_at` | TEXT | NOT NULL | 発行日時 |

**ビジネス制約:** `UNIQUE(sprint_id, sent_by)` — 1 スプリントにつき 1 人 1 回のみ発行可。

#### Post

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `notification_id` | TEXT | NOT NULL, FK → be_time_notifications | |
| `user_id` | TEXT | NOT NULL, FK → users | |
| `group_id` | TEXT | NOT NULL, FK → groups | フィード取得用（非正規化） |
| `repo_name` | TEXT | | リポジトリ名 |
| `branch_name` | TEXT | | ブランチ名 |
| `commit_count` | INTEGER | DEFAULT 0 | 今日のコミット数 |
| `diff_add` | INTEGER | DEFAULT 0 | 追加行数 |
| `diff_remove` | INTEGER | DEFAULT 0 | 削除行数 |
| `commit_message` | TEXT | | 最新コミットメッセージ（ユーザー上書き可） |
| `memo` | TEXT | | ひとことメモ |
| `privacy_level` | TEXT | NOT NULL, CHECK `public\|group\|private` | 投稿範囲 |
| `status` | TEXT | NOT NULL, CHECK `on_time\|late\|missed` | 投稿期限ステータス |
| `created_at` | TEXT | NOT NULL | 投稿日時 |

**ビジネス制約:** `UNIQUE(notification_id, user_id)` — 1 通知 × 1 ユーザー = 最大 1 投稿。

**status 算出ルール:**

| status | 条件 |
|--------|------|
| `on_time` | `created_at` ≤ `sent_at` + 1 時間（INSERT 時に算出） |
| `late` | `created_at` > `sent_at` + 1 時間（INSERT 時に算出） |
| `missed` | 投稿なし — スプリント終了バッチで `status='missed'` の Post を upsert |

#### Tag / PostTag

| テーブル | カラム | 制約 |
|---------|--------|------|
| `tags` | `id` TEXT PK, `name` TEXT NOT NULL UNIQUE | 技術タグマスタ |
| `post_tags` | `post_id` FK, `tag_id` FK, PK(post_id, tag_id) | 投稿 ↔ タグ |

#### Photo

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `post_id` | TEXT | NOT NULL, FK → posts | |
| `r2_key` | TEXT | NOT NULL | R2 オブジェクトキー（URL は API 層で署名生成） |
| `type` | TEXT | NOT NULL, CHECK `code_screenshot\|desk\|environment` | 写真種別 |
| `blur` | INTEGER | NOT NULL, CHECK 0\|1 | ぼかし適用 |

#### Reaction

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `post_id` | TEXT | NOT NULL, FK → posts | |
| `user_id` | TEXT | NOT NULL, FK → users | |
| `type` | TEXT | NOT NULL, CHECK `lgtm\|watching\|grass\|strong\|review\|merge` | リアクション種別 |

**ビジネス制約:** `UNIQUE(post_id, user_id, type)` — 種別ごとに 1 つ。

#### Comment

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | TEXT | PK | UUID |
| `post_id` | TEXT | NOT NULL, FK → posts | |
| `user_id` | TEXT | NOT NULL, FK → users | |
| `body` | TEXT | NOT NULL | コメント本文 |
| `created_at` | TEXT | NOT NULL | 作成日時 |

#### GitHubWebhookDelivery（冪等性）

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `delivery_id` | TEXT | PK | GitHub `X-GitHub-Delivery` ヘッダー |
| `event_type` | TEXT | NOT NULL | イベント種別 |
| `received_at` | TEXT | NOT NULL | 受信日時 |

### 6.2 ER 図

ER 図・ドメイン構造・データフローは [docs/database-er.md](docs/database-er.md) を参照。

### 6.3 主要インデックス

| インデックス | 対象 | 用途 |
|-------------|------|------|
| `idx_fcm_tokens_user_id` | fcm_tokens(user_id) | FCM 送信 |
| `idx_groups_created_by` | groups(created_by) | ユーザー作成グループ |
| `idx_groups_repo_full_name` | groups(repo_full_name) | リポジトリ名検索 |
| `idx_group_members_user_id` | group_members(user_id) | ユーザー所属グループ |
| `idx_group_members_group_active` | group_members(group_id, left_at) | 在籍メンバー一覧 |
| `idx_sprints_group_active` | sprints(group_id, ends_at) | 現行スプリント特定 |
| `idx_be_time_notifications_group` | be_time_notifications(group_id, sent_at DESC) | 通知履歴 |
| `idx_be_time_notifications_sprint_id` | be_time_notifications(sprint_id) | スプリント内通知 |
| `idx_posts_group_feed` | posts(group_id, created_at DESC) | フィード取得 |
| `idx_posts_user` | posts(user_id) | ユーザー投稿一覧 |
| `idx_posts_notification_id` | posts(notification_id) | Missed バッチ / 通知別投稿 |
| `idx_post_tags_tag_id` | post_tags(tag_id) | タグ別投稿 |
| `idx_photos_post_id` | photos(post_id) | 写真取得 |
| `idx_reactions_post_id` | reactions(post_id) | リアクション取得 |
| `idx_comments_post_id` | comments(post_id, created_at) | コメント取得 |

> **Note:** DB 上のテーブル名 `groups` は SQL 予約語のため、クエリでは `"groups"` とクォートする。

### 6.4 マイグレーション

- 配置: `backend/migrations/0001_initial.sql`
- 検証: `make db-validate` または `cd backend && npm run db:validate`
- ローカル適用: `make db-migrate-local` または `cd backend && npm run db:migrate:local`
- 本番適用: `wrangler d1 migrations apply begit-db --remote`（`database_id` 設定後）
- 命名規則: `{seq}_{action}_{object}`（例: `0002_add_follows_table.sql`）

---

## 7. API設計（概要）

### 認証

| Method | Path | 説明 |
|--------|------|------|
| `POST` | `/auth/github` | GitHubトークン交換・ログイン |
| `DELETE` | `/auth/logout` | ログアウト |

### グループ

| Method | Path | 説明 |
|--------|------|------|
| `POST` | `/groups` | グループ作成（リポジトリ指定あり） |
| `GET` | `/groups/:id` | グループ情報取得 |
| `POST` | `/groups/:id/sync-members` | リポジトリのコラボレーター情報を再同期してメンバー自動追加 |
| `DELETE` | `/groups/:id/members/me` | グループ脱退 |
| `POST` | `/groups/:id/notifications` | 通知発行 |

### デバイストークン

| Method | Path | 説明 |
|--------|------|------|
| `POST` | `/devices` | FCM registration token を登録・更新 |
| `DELETE` | `/devices/:token` | デバイストークン削除（ログアウト時） |

### 投稿

| Method | Path | 説明 |
|--------|------|------|
| `POST` | `/posts` | 投稿作成 |
| `GET` | `/groups/:id/feed` | フィード取得 |
| `GET` | `/posts/:id` | 投稿詳細 |
| `POST` | `/posts/:id/reactions` | リアクション追加 |
| `POST` | `/posts/:id/comments` | コメント追加 |

### GitHub連携

| Method | Path | 説明 |
|--------|------|------|
| `GET` | `/github/repos` | 最近のリポジトリ一覧 |
| `GET` | `/github/commits` | 最近のコミット情報 |
| `POST` | `/github/webhook` | commitイベント受信 |

---

## 8. 技術スタック

| レイヤー | 技術 | 備考 |
|--------|------|------|
| iOS アプリ | Swift / SwiftUI | ターゲット: iOS 16+ |
| バックエンド | Go | Workers Containers (linux/amd64) |
| クラウド | Cloudflare Workers | エントリーポイント・ルーティング |
| DB | Cloudflare D1 | SQLite 互換 |
| Push通知 | FCM (Firebase Cloud Messaging) → APNs | Go → FCM HTTP API → APNs → iPhone |
| GitHub連携 | GitHub REST API v3 / Webhooks | |
| 認証 | GitHub OAuth 2.0 | |
| ストレージ | Cloudflare R2 | S3互換API、写真保存用 |
| IaC | Terraform (cloudflare provider) + Wrangler | リソース作成は Terraform、デプロイは Wrangler |

---

## 9. 非機能要件

| 項目 | 要件 |
|------|------|
| セキュリティ | GitHubアクセストークンはサーバーサイドで暗号化保存 |
| パフォーマンス | フィード初回ロード 2秒以内 |
| プライバシー | Private repoアクセスはユーザーの明示的許可が必要 |
| 可用性 | ハッカソン期間中は99%以上 |
| プッシュ通知 | 通知発行から全員への配信を5秒以内 |

---

## 10. MVP スコープ

ハッカソン提出時点での実装目標:

- [x] GitHub OAuthログイン
- [x] グループ作成・参加
- [x] 通知発行（1スプリント1人1回）
- [x] GitHubコミット情報の自動取得と投稿
- [x] 写真添付（作業環境 / コード）
- [x] フィード表示（投稿前ぼかし → 投稿後詳細解放）
- [x] リアクション
- [x] On Time / Late / Missed ステータス
- [x] プライバシー設定
- [ ] PR / Issue 詳細連携（将来）
- [ ] フォロワー機能（将来）

---

## 11. 将来拡張

- PR / Issue の通知・レビューフロー連携
- スプリントのふりかえり機能（草グラフ、On Time率）
- Androidアプリ
- Slack / Discord 連携通知
- チームの貢献度ランキング・バッジシステム
- AI によるコミットメッセージサジェスト

---

*BeGit — あなたの開発を、チームのワクワクに。*
