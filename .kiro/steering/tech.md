# Technology Stack

## Architecture

iOS クライアント ↔ Go REST API ↔ PostgreSQL / AWS S3 / APNs / GitHub API

GitHub Webhook でコミット・PR レビューをリアルタイム検知し、APNs 経由でスマホへ Push 通知を送る。

## Core Technologies

| レイヤー | 技術 |
|---|---|
| **iOS** | Swift / SwiftUI (iOS 16+) |
| **バックエンド** | Go |
| **DB** | PostgreSQL |
| **認証** | GitHub OAuth 2.0 |
| **Push 通知** | APNs (HTTP/2) |
| **GitHub 連携** | REST API v3 / Webhooks |
| **ストレージ** | AWS S3 |

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

# バックエンド起動
cd backend && go run cmd/server/main.go

# iOS
open ios/BeGit/BeGit.xcodeproj  # Xcode で ⌘R
```

## Key Technical Decisions

- **GitHub OAuth をバックエンド経由に集約**: アクセストークンをクライアントに持たせず、サーバーサイドで暗号化保存
- **グループ参加の自動化**: `GET /repos/{owner}/{repo}/collaborators` を使い、手動招待なしで参加させる
- **ぼかし制御をサーバー側で**: フィード API がリクエストユーザーの投稿状況を見てレスポンスを制御

---
_Document standards and patterns, not every dependency_
