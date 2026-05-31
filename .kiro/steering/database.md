# Database Standards (Cloudflare D1)

BeGit のサーバーサイド DB 設計指針。D1 は SQLite 互換方言を使用する。

## 方針

- ドメインを先にモデル化し、正しさを優先してから最適化する
- 不変条件は DB 制約（NOT NULL / UNIQUE / CHECK / FK）で明示する
- マイグレーションは immutable — 変更は新規 migration で追加する

## 命名規則

| 対象 | 規則 | 例 |
|------|------|-----|
| テーブル | snake_case, 複数形 | `users`, `be_time_notifications` |
| カラム | snake_case | `created_at`, `user_id` |
| FK カラム | `{table_singular}_id` | `group_id` → `groups.id` |
| インデックス | `idx_{table}_{columns}` | `idx_posts_group_feed` |

## 型

| 用途 | D1/SQLite 型 |
|------|-------------|
| ID | `TEXT`（UUID v4、Go/Swift 側で生成） |
| 日時 | `TEXT` ISO8601 UTC（`datetime('now')` デフォルト可） |
| 真偽値 | `INTEGER` 0/1 + CHECK |
| 列挙 | `TEXT` + CHECK 制約 |

## リレーションシップ

- **1:N** — 子テーブルに FK
- **N:N** — junction テーブル + 複合 PK（例: `post_tags`）
- **中間テーブル（GroupMember 等）** — 複合 PK を基本とし、不要な surrogate `id` は避ける

## マイグレーション

```bash
# ローカル適用
wrangler d1 migrations apply begit-db --local

# 本番適用
wrangler d1 migrations apply begit-db
```

- 配置: `backend/migrations/`
- 命名: `{seq}_{action}_{object}.sql`（例: `0001_initial.sql`, `0002_add_follows_table.sql`）
- 1 ファイル = 1 論点。ロールバック用 SQL をコメントで残す

## インデックス基準

以下は原則として INDEX を張る:

- すべての FK カラム
- フィード・一覧 API の `WHERE` + `ORDER BY` 列（例: `posts(group_id, created_at DESC)`）
- UNIQUE 制約の検索列（`github_login`, `registration_token`）

## トランザクション

- 複数テーブルへの書き込み（グループ作成 + owner を GroupMember 追加 + 初回 Sprint 作成等）は 1 トランザクションで実行
- D1 は SQLite 互換のため、書き込み単位は短く保つ

## BeGit 固有ルール

| ルール | 実装 |
|--------|------|
| 1 スプリント 1 人 1 通知 | `UNIQUE(sprint_id, sent_by)` on `be_time_notifications` |
| 1 通知 1 人 1 投稿 | `UNIQUE(notification_id, user_id)` on `posts` |
| GitHub トークン | `users.access_token_encrypted` — `DB_ENCRYPTION_KEY` で暗号化、平文保存禁止 |
| 写真 | DB には `photos.r2_key` のみ。公開 URL は API 層で署名生成 |
| Push 通知 vs BeGit Time 通知 | FCM = Push 配信。`be_time_notifications` = ドメイン上の「BeGit Time」通知発行 |
| MVP プライバシー | `privacy_level IN ('public','group','private')` — `followers` は将来 migration |
| MVP グループ:リポジトリ | 1:1 — `groups.repo_full_name` に直接保持 |
| Webhook 冪等性 | `github_webhook_deliveries.delivery_id` で重複排除 |
| SQL 予約語 | テーブル `groups` はクエリで `"groups"` とクォート |

## クエリパターン

| API | クエリ |
|-----|--------|
| フィード | `SELECT ... FROM posts WHERE group_id = ? ORDER BY created_at DESC LIMIT ?` |
| 通知発行可否 | `SELECT 1 FROM be_time_notifications WHERE sprint_id = ? AND sent_by = ?` |
| 自動参加 sync | `SELECT id FROM users WHERE github_login IN (...)` |
| Missed 付与 | スプリント終了時、未投稿メンバーへ `status='missed'` Post を upsert |

## ER 図

詳細は [docs/database-er.md](../../docs/database-er.md) を参照。

---
_D1/SQLite 方言に限定。PostgreSQL 固有機能は使用しない。_
