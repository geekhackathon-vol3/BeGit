<div align="center">

<!-- ここに画像を挿入: トップバナー横長画像 (docs/assets/banner.png) -->
<!-- ![BeGit Banner](docs/assets/banner.png) -->

<br>

# 🌸 BeGit

## *あなたの開発を、チームのワクワクに。*

BeReal × GitHub — 開発者のための瞬間シェア SNS

<br>

[![Swift](https://img.shields.io/badge/Swift-5.9-FF6B9D?style=for-the-badge&logo=swift&logoColor=white)](https://swift.org)
[![SwiftUI](https://img.shields.io/badge/SwiftUI-iOS16+-C9B8FF?style=for-the-badge&logo=apple&logoColor=white)](https://developer.apple.com/xcode/swiftui/)
[![Go](https://img.shields.io/badge/Go-Backend-7EC8E3?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![GitHub OAuth](https://img.shields.io/badge/Auth-GitHub_OAuth-FFB3C6?style=for-the-badge&logo=github&logoColor=white)](https://docs.github.com/en/apps/oauth-apps)

<br>

✨ 技育CAMP ハッカソン2026 vol.3 出展作品 ✨

</div>

---

## 概要

> **「今なに作ってる？」をチームで共有する、開発者のための瞬間シェア SNS**

リモートチームでの開発中、「みんな今何してるんだろう...」って思うことありませんか？  
Slackのスタンドアップは義務感があって続かない。そんな課題を、ゲームみたいな楽しさで解決するのが BeGit です。

BeReal の「通知が来たら今すぐ投稿！」をそのまま GitHub × チーム開発に持ち込みました。

---

## スクリーンショット

<!-- ここに画像を挿入: アプリのスクリーンショット4枚 (docs/assets/screenshots.png) -->

<div align="center">

| フィード | 投稿作成 | 投稿カード | 通知発行 |
|:-:|:-:|:-:|:-:|
| <!-- docs/assets/ss_feed.png --> | <!-- docs/assets/ss_create.png --> | <!-- docs/assets/ss_card.png --> | <!-- docs/assets/ss_notify.png --> |

</div>

---

## ゲームの流れ

```
📢 誰かが「BeGit Time！」を発行
        ↓
⏱️ 全員に通知 — 1時間のカウントダウン開始！
        ↓
💻 コードを書いて → コミットして → 投稿する！
        ↓
✅ On Time  /  😅 Late  /  💀 Missed
```

- 1スプリント **1人1回**しか通知を打てない → 「いつ打つか」の戦略性がカギ
- 自分が投稿するまで他メンバーの内容が**ぼかされる** → 投稿したくなる仕組み

---

## 主な機能

### GitHub 自動連携

投稿時に GitHub API からコミット情報を自動取得。ほぼワンタップで今の開発状況を投稿できます。

| 自動取得項目 | 内容 |
|:--|:--|
| Repository / Branch | 最近コミットしたものを自動サジェスト |
| 今日のコミット数 / 変更行数 | その日の作業量を可視化 |
| 最新コミットメッセージ | デフォルト表示・上書き可 |
| Open PR / Assigned Issue | GitHub から自動連携 |

### フィード & リアクション

投稿カードにはコミット情報・写真・技術タグが表示されます。開発者向け絵文字リアクションでサクッとフィードバック！

👍 LGTM　👀 見てる　🌱 草　💪 強い　📝 レビュー待ち？　🚀 Mergeしろ

### グループ & 自動参加

リポジトリを指定してグループを作ると、コラボレーターとして登録されている BeGit ユーザーが**自動でグループに参加**。招待の手間なし！

---

## システム構成

<!-- ここに画像を挿入: システム構成図 (docs/assets/architecture.png) -->

```
📱 iOSアプリ (SwiftUI)
        │ HTTPS
        ▼
[Cloudflare Workers]──────────── GitHub API
        │
[Workers Container (Go)]
        ├── GitHub Webhook → 署名検証 → FCM → APNs → iPhone
        ├── Cloudflare D1 (SQLite)
        └── Cloudflare R2 (写真)
```

---

## 技術スタック

| レイヤー | 技術 |
|:--|:--|
| iOS アプリ | Swift / SwiftUI (iOS 16+) |
| バックエンド | Go (Cloudflare Workers Containers) |
| クラウド | Cloudflare Workers |
| データベース | Cloudflare D1 (SQLite 互換) |
| 認証 | GitHub OAuth 2.0 |
| Push 通知 | FCM (Firebase Cloud Messaging) → APNs |
| GitHub 連携 | GitHub REST API v3 / Webhooks |
| ストレージ | Cloudflare R2 |

---

## 実装状況

| 機能 | 状態 |
|:--|:--:|
| GitHub OAuth ログイン | ✅ |
| グループ作成・自動参加 | ✅ |
| 通知発行（1スプリント1人1回） | ✅ |
| GitHub コミット情報の自動取得と投稿 | ✅ |
| 写真添付（作業環境 / コード） | ✅ |
| フィード（投稿前ぼかし → 投稿後解放） | ✅ |
| リアクション / コメント | ✅ |
| On Time / Late / Missed ステータス | ✅ |
| プライバシー設定 | ✅ |
| PR / Issue 詳細連携 | 🔜 将来実装 |

---

## セットアップ

### 初回セットアップ（git hooks）

```bash
make setup
```

これで `git commit` 時にパッケージ変更を検出し、Claude Code が自動で README を更新します（Claude Code インストール済みの場合のみ動作）。

### 必要な環境

- Go 1.22+
- Xcode 15+
- Node.js 18+（wrangler CLI 用）
- Cloudflare アカウント
- GitHub OAuth App（[作成はこちら](https://github.com/settings/developers)）
- Firebase プロジェクト（FCM 用サービスアカウントキー）

### バックエンド

```bash
git clone https://github.com/geekhackathon-vol3/BeGit.git
cd BeGit/backend

# wrangler CLI インストール
npm install -g wrangler

# ローカル用シークレットの設定
cp .dev.vars.example .dev.vars
```

`.dev.vars` に以下を記入してください：

```
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret
GITHUB_WEBHOOK_SECRET=your_webhook_secret
FIREBASE_SERVICE_ACCOUNT_JSON={"type":"service_account",...}
DB_ENCRYPTION_KEY=your_encryption_key
```

```bash
# 依存関係のインストール & ローカルサーバー起動
go mod download
wrangler dev
```

### iOS アプリ

```bash
cd BeGit/ios/BeGit
open BeGit.xcodeproj
```

Xcode で以下を設定してください：

1. **Signing & Capabilities** で自分の Apple Developer アカウントを選択
2. **Bundle Identifier** を変更
3. `Config.xcconfig` にバックエンドの URL を記入
4. `⌘R` でビルド & 実行

### データベース

```bash
# マイグレーション実行
wrangler d1 migrations apply begit-db
```

---

## ディレクトリ構成

```
BeGit/
├── ios/
│   └── BeGit/
│       ├── BeGit/          # ソースコード
│       │   ├── Views/          # SwiftUI 画面
│       │   ├── Models/         # データモデル
│       │   ├── ViewModels/     # ViewModel
│       │   └── Services/       # API クライアント・GitHub連携
│       └── BeGit.xcodeproj
├── backend/
│   ├── cmd/
│   │   └── server/         # エントリーポイント
│   ├── internal/
│   │   ├── handler/        # HTTPハンドラー
│   │   ├── service/        # ビジネスロジック
│   │   └── repository/     # DB アクセス
│   └── pkg/
│       ├── fcm/            # FCM クライアント
│       └── github/         # GitHub API クライアント
└── infra/
    ├── infra.md            # インフラ構成
    └── infra.mermaid       # 構成図ソース
```

---

## チーム

<!-- ここに画像を挿入: チームの集合写真 (docs/assets/team.png) -->

| メンバー | 役割 |
|:--|:--|
| Palm | 鬼がかったSwifter👹 |
| Ochitomo | 天才frontend🌟 |
| Riri | GoGo💨Backend |
| Riochin | パワー‼️infra‼️ |

---

<div align="center">

✨ **技育CAMP ハッカソン2026 vol.3** ✨

*BeGit — あなたの開発を、チームのワクワクに。* 🌸

</div>
