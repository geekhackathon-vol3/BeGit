# BeGit インフラ構成

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
    [APNs]               ← Push通知
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
| **APNs** | iOSへのPush通知 | GoからHTTP/2で直接送信 |
| **GitHub Webhooks** | commit / PRイベント受信 | HMAC-SHA256で署名検証 |

---

## 管理するシークレット

| シークレット | 用途 |
|------------|------|
| `GITHUB_CLIENT_SECRET` | GitHub OAuth |
| `GITHUB_WEBHOOK_SECRET` | Webhook署名検証 |
| `APNS_PRIVATE_KEY` | APNs認証 (.p8) |
| `APNS_KEY_ID` | APNs Key ID |
| `APNS_TEAM_ID` | Apple Developer Team ID |
| `DB_ENCRYPTION_KEY` | GitHubアクセストークンの暗号化 |

---

## デプロイフロー

```
1. Dockerfileをビルド
2. wrangler deploy
   └─ Cloudflareレジストリにイメージをpush
   └─ Workersをデプロイ
   └─ Containerをプロビジョニング（初回数分かかる）
3. wrangler d1 migrations apply  ← DBマイグレーション
```

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

Cloudflare外で依存するのは以下の2つのみ。

| サービス | 用途 |
|---------|------|
| **APNs** (Apple) | iOSへのPush通知 |
| **GitHub** | OAuth認証 / Webhooks / REST API |

---

## 補足

- ドメイン・HTTPSはCloudflare DNSで自動管理
- CDNはCloudflareがそのまま兼ねる（別途不要）
- GitHub Webhook受信には公開URLが必要（Workers ContainersはデフォルトでパブリックURL付き）
- コンテナはアイドル時に`sleepAfter`でスリープするため、コールドスタートに注意（デモ前にウォームアップ推奨）
