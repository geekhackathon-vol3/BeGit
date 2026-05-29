# Technology Stack

## Architecture

iOS クライアント ↔ Cloudflare Workers ↔ Workers Container (Go) ↔ D1 / R2 / APNs / GitHub API

GitHub Webhook でコミット・PR レビューをリアルタイム検知し、APNs 経由でスマホへ Push 通知を送る。

## Core Technologies

| レイヤー | 技術 |
|---|---|
| **iOS** | Swift / SwiftUI (iOS 16+) |
| **バックエンド** | Go (Cloudflare Workers Containers, linux/amd64) |
| **エントリーポイント** | Cloudflare Workers（ルーティング） |
| **DB** | Cloudflare D1（SQLite 互換） |
| **ストレージ** | Cloudflare R2（S3 互換 API） |
| **シークレット管理** | Cloudflare Workers Secrets |
| **認証** | GitHub OAuth 2.0 |
| **Push 通知** | APNs (HTTP/2) |
| **GitHub 連携** | REST API v3 / Webhooks |
| **IaC** | Terraform (cloudflare provider) + Wrangler |

## Development Standards

### iOS (Swift/SwiftUI)
- アーキテクチャ: MVVM（Views / ViewModels / Models / Services）
- ターゲット: iOS 16+
- GitHub OAuth: `ASWebAuthenticationSession` で実装
- Push 通知受信: `AppDelegate.didReceiveRemoteNotification` で `type` フィールドを判定し画面遷移

### Backend (Go)
- レイヤード構成: `cmd/` (エントリーポイント) / `internal/` (ビジネスロジック) / `pkg/` (外部連携)
- Webhook セキュリティ: `X-Hub-Signature-256` を HMAC-SHA256 で必ず検証。失敗時は `403` を返す

### Development Environment

```bash
# セットアップ（git hooks 有効化）
make setup

# バックエンド起動（ローカル）
cd backend && wrangler dev

# DBマイグレーション
wrangler d1 migrations apply begit-db

# iOS
open ios/BeGit/BeGit.xcodeproj  # Xcode で ⌘R
```

## Key Technical Decisions

- **Cloudflare Workers + Workers Containers**: Workers がエントリーポイント・ルーティングを担い、Go コンテナがビジネスロジックを処理。コールドスタートに注意（デモ前にウォームアップ推奨）
- **D1 は SQLite 互換**: PostgreSQL ではなく SQLite の方言で SQL を書く。マイグレーションは `wrangler d1 migrations apply`
- **GitHub OAuth をバックエンド経由に集約**: アクセストークンをクライアントに持たせず、Workers Secrets + `DB_ENCRYPTION_KEY` で暗号化保存
- **グループ参加の自動化**: `GET /repos/{owner}/{repo}/collaborators` を使い、手動招待なしで参加させる
- **ぼかし制御をサーバー側で**: フィード API がリクエストユーザーの投稿状況を見てレスポンスを制御

---
_Document standards and patterns, not every dependency_
