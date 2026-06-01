# フロント向け Dev API ガイド

開発環境（dev）の API を、**ローカル環境構築ゼロ**で叩くためのガイドです。
`.envrc` も Go も wrangler も Cloudflare アカウントも **不要**。共有された dev URL を叩くだけです。

---

## TL;DR

```bash
BASE=https://begit-dev.118029-ichikama.workers.dev

# 1. dev ログインでトークン取得（GitHub OAuth 不要）
curl -X POST $BASE/auth/dev -H 'Content-Type: application/json' -d '{"login":"alice"}'
# → {"user":{...},"token":"dev_alice"}

# 2. 以降は Authorization: Bearer dev_alice で全 API を叩ける
curl $BASE/groups -H "Authorization: Bearer dev_alice"
```

固定トークンなので、`/auth/dev` を毎回叩かずに **`dev_alice` をそのまま使う**のでも OK です
（ただしユーザーを DB に作るため、初回は一度 `/auth/dev` を叩いてください）。

---

## 認証（dev）

| 項目 | 内容 |
|---|---|
| ログイン | `POST /auth/dev`、body `{"login":"alice"}`（省略時 alice） |
| 既定ユーザー | `alice` / `bob` / `carol`（任意の login も可） |
| トークン | `dev_<login>`（例: `dev_alice`、`dev_bob`） |
| 使い方 | 全ての認証必須 API に `Authorization: Bearer dev_alice` を付与 |

> dev では GitHub 情報（コミット数・変更行数など）は**スタブ（固定値）**が返ります。
> 実 GitHub には接続しません。本番の `POST /auth/github`（OAuth）とは別物です。

---

## エンドポイント一覧

| メソッド | パス | 認証 | 説明 |
|---|---|---|---|
| GET  | `/healthz` | 不要 | 疎通確認（`{"status":"ok"}`） |
| POST | `/auth/dev` | 不要 | **dev 専用**ログイン → トークン発行 |
| GET  | `/groups` | Bearer | 所属グループ一覧 |
| POST | `/groups` | Bearer | グループ作成（collaborator 自動参加） |
| GET  | `/groups/{id}` | Bearer + メンバー | グループ詳細＋メンバー |
| POST | `/groups/{id}/notifications` | メンバー | 通知発行（1スプリント1人1回） |
| GET  | `/groups/{id}/notifications/{nid}` | メンバー | 通知ステータス |
| POST | `/groups/{id}/posts` | メンバー | 投稿作成（GitHub 情報はスタブ） |
| GET  | `/groups/{id}/posts` | メンバー | フィード取得（未投稿はぼかし制御） |
| PUT  | `/me/fcm-token` | Bearer | FCM トークン登録 |

---

## curl サンプル一式

```bash
BASE=https://begit-dev.118029-ichikama.workers.dev
TOKEN=dev_alice

# グループ作成（repo_full_name は任意の文字列で OK。dev では実在不要）
curl -X POST $BASE/groups -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"repo_full_name":"begit-dev/playground","name":"Dev Playground"}'

# グループ一覧 / 詳細
curl $BASE/groups -H "Authorization: Bearer $TOKEN"
curl $BASE/groups/1 -H "Authorization: Bearer $TOKEN"

# 通知発行
curl -X POST $BASE/groups/1/notifications -H "Authorization: Bearer $TOKEN"

# 投稿作成（github_login は自分の login、repo_full_name はグループの repo）
curl -X POST $BASE/groups/1/posts -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"body":"作業中！","notification_id":1,"github_login":"alice","repo_full_name":"begit-dev/playground"}'

# フィード取得
curl $BASE/groups/1/posts -H "Authorization: Bearer $TOKEN"
```

> すぐ試したい場合、すでにシード済みのグループ／投稿が入っています（`make seed-dev` 実行済みの場合）。
> 空なら上記の手順で作成してください。

---

## iOS から繋ぐ

`Config.xcconfig` にバックエンド URL を設定します（README「iOS アプリ」参照）。

```
API_BASE_URL = https:/$()/begit-dev.118029-ichikama.workers.dev
```

> `xcconfig` では `//` がコメント扱いになるため、`https:/$()/...` のように
> `$()` を挟んでエスケープするか、ビルド設定側で URL を組み立ててください。

アプリ側は `/auth/dev` で取得した `token` を `Authorization: Bearer <token>` ヘッダーに付けて
各 API を呼び出します（本番の OAuth フローに差し替わるまでの開発用）。

---

## 補足

- **`.envrc` は不要**: それが必要なのは API を**起動・デプロイする側**（バックエンド/インフラ担当）だけです。
- dev は本番とは**別 Worker（begit-dev）＋別 D1（begit-db-dev）**に隔離されています。dev で作ったデータが本番に影響することはありません。
- dev の URL・トークンが変わった場合はバックエンド担当に確認してください。
