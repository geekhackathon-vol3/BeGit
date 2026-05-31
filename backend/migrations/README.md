# D1 Migrations

BeGit の Cloudflare D1 スキーママイグレーション。

## ファイル

| ファイル | 内容 |
|---------|------|
| `0001_initial.sql` | 初期スキーマ（13 テーブル） |

## レビュー記録（2026-05-31）

| 項目 | 対応 |
|------|------|
| `groups` は SQL 予約語 | テーブル名を `"groups"` とクォート |
| FK 列のインデックス不足 | `sprint_id`, `notification_id`, `tag_id`, `left_at` 等を追加 |
| スプリント期間の整合性 | `CHECK (ends_at > started_at)` |
| 空コメント防止 | `comments.body` に `length(trim(body)) > 0` |
| 負数カウント防止 | `posts.commit_count/diff_*` に `>= 0` CHECK |

## コマンド

```bash
cd backend

# 1. 依存関係
npm install

# 2. SQLite で検証（Cloudflare アカウント不要）
npm run db:validate

# 3. ローカル D1 に適用
npm run db:migrate:local

# 4. 本番 D1 に適用（database_id 設定後）
npm run db:migrate:remote
```

## 初回セットアップ（Cloudflare）

```bash
wrangler d1 create begit-db
# 出力された database_id を wrangler.toml に設定
npm run db:migrate:remote
```

## 注意

- クエリで `groups` テーブルを参照するときは `"groups"` とクォートする
- `database_id` のプレースホルダは本番デプロイ前に差し替える
