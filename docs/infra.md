# BeGit; インフラ構成

**バージョン:** 1.0.0
**作成日:** 2026-05-28

---

## 構成図

```
iOSアプリ
    ↓↑
[Cloudflare Workers]         ← エントリーポイント・ルーティング
    ↓↑                ↓              ↓
[Workers Container]        [D1]          [R2]
      (Go)              (メインDB)    (写真ストレージ)
      ↓
    [FCM]                ← Push通知 (Firebase Cloud Messaging)
      ↓
    [APNs]               ← FCM → APNs → iPhone
      ↑
[GitHub Webhooks]          ← commit / PRイベント受信
```

---

## サービス一覧

| サービス | 用途 | 備考 |
|---------|------|------|
| **Cloudflare Workers** | エントリーポイント、ルーティング | wrangler管理 |
| **Workers Containers (Go)** | APIロジック本体 | Dockerイメージ、linux/amd64 |
| **Cloudflare D1** | メインDB | SQLite互換 |
| **Cloudflare R2** | 写真ストレージ | S3互換API、egress無料 |
| **Cloudflare Workers Secrets** | シークレット管理 | 下記参照 |
| **FCM** (Firebase Cloud Messaging) | iOSへのPush通知中継 | Go → FCM → APNs → iPhone |
| **GitHub Webhooks** | commit / PRイベント受信 | HMAC-SHA256で署名検証 |

---

## 管理するシークレット

| シークレット | 用途 |
|------------|------|
| `GITHUB_CLIENT_SECRET` | GitHub OAuth |
| `GITHUB_WEBHOOK_SECRET` | Webhook署名検証 |
| `FIREBASE_SERVICE_ACCOUNT_JSON` | FCM 認証（サービスアカウントキー） |
| `DB_ENCRYPTION_KEY` | GitHubアクセストークンの暗号化 |

---

## 環境（PoC 期間）

| 環境 | Worker / D1 | 用途 | デプロイ |
|------|-------------|------|---------|
| dev | `begit-dev` / `begit-db-dev` | 運用環境。TestFlight と GitHub App の接続先。alice / bob のシードも同居 | main へのマージで自動（`deploy-dev.yml`） |
| 本番 | `begit` / `begit-db` | 利用者なし。App Store 公開時に切り替える | 手動（`deploy.yml`） |

- 手元から `wrangler deploy` / `make deploy-dev` はしない。dev に出ている内容を main と一致させるため。
- ブランチを dev で試すときは Actions の "Deploy (dev)" を ref 指定で実行し、確認後すぐにマージする。
- dev への自動デプロイは `backend/` の変更時のみ。デプロイ前に `go test ./...` を実行し、失敗したら出さない。
- 戻すときは revert PR をマージする。マイグレーションは追加のみなので DB は戻さない。

## デプロイフロー

```
1. wrangler d1 migrations apply  ← 先にスキーマを更新（追加のみのマイグレーション）
2. wrangler deploy
   └─ Dockerfileをビルドし、Cloudflareレジストリにイメージをpush
   └─ Workersをデプロイ
   └─ Containerをプロビジョニング（初回数分かかる）
```

マイグレーションは新コードのデプロイ前に適用されるため、旧コードでも動く形（テーブル・列の追加）に限る。列の削除や名前変更は「追加 → 移行 → 削除」に分けて出す。

### ローカル開発

```bash
wrangler dev        # Workersのローカル実行
wrangler d1 execute # D1のローカル操作
```

---

## IaC

Terraformと Wranglerを併用する。

| 管理対象 | ツール |
|---------|------|
| D1 / R2 / Workers のリソース作成 | Terraform (`cloudflare` provider) |
| Workers / Containerのデプロイ | Wrangler |
| DBマイグレーション | Wrangler (`wrangler d1 migrations apply`) |

---

## 外部依存サービス

Cloudflare外で依存するのは以下の3つ。

| サービス | 用途 |
|---------|------|
| **FCM** (Firebase Cloud Messaging) | Push通知の中継（FCM → APNs → iPhone） |
| **APNs** (Apple) | FCM 経由で最終配信（直接管理不要） |
| **GitHub** | OAuth認証 / Webhooks / REST API |

---

## 補足

- ドメイン・HTTPSはCloudflare DNSで自動管理
- CDNはCloudflareがそのまま兼ねる（別途不要）
- GitHub Webhook受信には公開URLが必要（Workers ContainersはデフォルトでパブリックURL付き）
- コンテナはアイドル時に`sleepAfter`でスリープするため、コールドスタートに注意（デモ前にウォームアップ推奨）
