# Project Structure

## Organization Philosophy

モノレポ構成。`ios/` と `backend/` がそれぞれ独立したプロジェクト。共有コードはなく、API 契約（REST）で連携する。

## Directory Patterns

### iOS アプリ (`ios/BeGit/BeGit/`)
**Purpose**: SwiftUI アプリ本体。MVVM パターンで層分け  
**Convention**:
- `Views/` — SwiftUI 画面コンポーネント
- `ViewModels/` — 画面ロジック・状態管理
- `Models/` — データモデル（Codable）
- `Services/` — API クライアント・GitHub 連携・APNs 登録

### Go バックエンド (`backend/`)
**Purpose**: REST API サーバー。Clean Architecture ライクな 3 層構成  
**Convention**:
- `cmd/server/` — エントリーポイント
- `internal/handler/` — HTTP ハンドラー（リクエスト/レスポンス整形）
- `internal/service/` — ビジネスロジック
- `internal/repository/` — DB アクセス
- `pkg/apns/` — APNs クライアント
- `pkg/github/` — GitHub API クライアント

### Git 自動化 (`.githooks/`)
**Purpose**: コミット時の自動処理  
**Convention**: `post-commit` フックでパッケージ関連ファイルの変更を検出し、Claude Code で README を自動更新。`make setup` で有効化。

### CI/CD (`.github/workflows/`)
**Purpose**: PR への自動ラベル付け（`labeler.yml` で変更ファイルパスからラベルを決定）

## Naming Conventions

- **Swift ファイル / 型**: PascalCase（例: `FeedViewModel.swift`）
- **Swift 関数 / プロパティ**: camelCase
- **Go ファイル**: snake_case（例: `webhook_handler.go`）
- **Go 型**: PascalCase、パッケージ外公開は大文字始まり
- **API エンドポイント**: REST 規則 (`/groups/:id/notifications`)

## Code Organization Principles

- iOS の `Services/` 層がバックエンドへの HTTP 通信を担い、ViewModel はサービスを呼ぶだけにする
- バックエンドの `handler → service → repository` の依存方向を守る（逆依存禁止）
- GitHub API アクセスは必ずバックエンド経由（iOS から直接叩かない）

---
_Document patterns, not file trees. New files following patterns shouldn't require updates_
