# 要件定義書

## プロジェクト概要入力

BeGit; のインフラ構築。Cloudflare Workers をエントリーポイントに、Workers Containers (Go, linux/amd64) でAPIを動かす。DB は Cloudflare D1 (SQLite互換)、写真ストレージは Cloudflare R2、シークレット管理は Workers Secrets。Push通知は FCM (Firebase Cloud Messaging) 経由で APNs に配信するため、Firebase プロジェクトのサービスアカウントキーをシークレットとして登録する（Firebase プロジェクト自体は Firebase コンソールで作成）。IaC は Terraform (cloudflare provider) でリソース作成、Wrangler でデプロイ・DBマイグレーション。詳細は infra/infra.md を参照。

## スコープ

- **対象**: Cloudflare プラットフォーム上のインフラリソースの作成・管理・デプロイ自動化
- **対象外**: Firebase プロジェクト本体の作成（Firebase コンソールで手動作成）、GitHub の Webhook 設定画面操作、iOS クライアント側の設定
- **隣接システム**: FCM（Push通知中継）、GitHub（OAuth / Webhooks）、APNs（FCM経由での最終配信）

---

## 要件

### 要件 1: Cloudflare Workers エントリーポイント設定

**目的:** インフラ管理者として、Cloudflare Workers をルーティングのエントリーポイントとして Terraform で管理したい。それにより、iOSクライアントからのリクエストを Workers Containers に確実に転送できる構成を Infrastructure as Code で表現できるようにする。

#### 受け入れ基準
1. The Cloudflare Workers shall D1・R2・Workers Containers への binding を wrangler.toml に定義する
2. When Terraform が実行される, the Cloudflare インフラ管理システム shall Workers リソースが作成または更新される
3. When Workers が iOSクライアントからリクエストを受信する, the Cloudflare Workers shall リクエストを Workers Container エンドポイントへ転送する
4. If Terraform の apply が失敗した場合, the Cloudflare インフラ管理システム shall エラー内容を標準出力に表示し、部分適用済みリソースの状態を state ファイルに記録する
5. The Cloudflare Workers shall Cloudflare の自動 HTTPS・CDN の恩恵を受け、別途 TLS 証明書管理を不要とする

---

### 要件 2: Workers Containers (Go API) デプロイ

**目的:** インフラ管理者として、Go バックエンドを Workers Containers として Wrangler でデプロイしたい。それにより、ビジネスロジック・DB・外部API連携をサーバーレスコンテナとして実行できるようにする。

#### 受け入れ基準
1. The デプロイシステム shall linux/amd64 アーキテクチャの Docker イメージを Cloudflare コンテナレジストリに push する
2. When `wrangler deploy` が実行される, the Wrangler shall Workers Container のプロビジョニングを行い、初回は完了まで数分を要する
3. The Workers Container shall コールドスタートを防ぐため `sleepAfter` 設定によるアイドルスリープ期間を設定可能とする
4. While Workers Container がリクエストを処理中, the Workers Container shall D1・R2・FCM・GitHub API へ各 binding・シークレットを通じてアクセスできる
5. If コンテナイメージのビルドに失敗した場合, the デプロイシステム shall デプロイを中断しエラーを出力する

---

### 要件 3: Cloudflare D1 データベース管理

**目的:** インフラ管理者として、Cloudflare D1 を Terraform で作成し、スキーマを Wrangler マイグレーションで管理したい。それにより、SQLite 互換の DB を IaC で再現可能な状態に保てるようにする。

#### 受け入れ基準
1. When Terraform が実行される, the Cloudflare インフラ管理システム shall `begit-db` という名前の D1 データベースを作成する
2. When `wrangler d1 migrations apply begit-db` が実行される, the Wrangler shall 未適用のマイグレーションファイルを順番に D1 へ適用する
3. If マイグレーションファイルの SQL に文法エラーがある場合, the Wrangler shall マイグレーションを中断しエラー内容を出力する
4. The D1 データベース shall SQLite 方言（PostgreSQL 方言不可）で記述されたマイグレーション SQL を受け付ける
5. The Cloudflare Workers shall wrangler.toml の binding 設定を通じて D1 データベースへアクセスできる

---

### 要件 4: Cloudflare R2 写真ストレージ設定

**目的:** インフラ管理者として、Cloudflare R2 を Terraform で作成したい。それにより、投稿写真を egress コストなしで保存・配信できるストレージ基盤を IaC で管理できるようにする。

#### 受け入れ基準
1. When Terraform が実行される, the Cloudflare インフラ管理システム shall `begit-photos` という名前の R2 バケットを作成する
2. The R2 バケット shall S3 互換 API を通じて Workers Container からオブジェクトのアップロード・取得が可能である
3. The Cloudflare Workers shall wrangler.toml の binding 設定を通じて R2 バケットへアクセスできる
4. The R2 バケット shall Cloudflare ネットワーク内での egress コストが発生しない設定で運用できる
5. If R2 へのアップロードが失敗した場合, the Workers Container shall エラーレスポンスをクライアントに返す

---

### 要件 5: Workers Secrets シークレット管理

**目的:** インフラ管理者として、4 種類のシークレットを Cloudflare Workers Secrets に登録したい。それにより、機密情報をソースコードやバージョン管理に含めることなく安全に管理できるようにする。

#### 受け入れ基準
1. The シークレット管理システム shall 以下の 4 つのシークレットを Workers Secrets に登録できる: `GITHUB_CLIENT_SECRET`・`GITHUB_WEBHOOK_SECRET`・`FIREBASE_SERVICE_ACCOUNT_JSON`・`DB_ENCRYPTION_KEY`
2. The Workers Container shall 実行時に Workers Secrets から各シークレット値を環境変数として参照できる
3. The シークレット管理システム shall シークレット値をソースコードや Terraform state に平文で記録しない
4. If シークレットが未登録の状態で Workers が起動した場合, the Cloudflare Workers shall 依存するエンドポイントで適切なエラーを返す
5. When 新しいシークレット値を設定する必要がある場合, the Wrangler shall `wrangler secret put <KEY>` コマンドで既存シークレットを上書き登録できる

---

### 要件 6: FCM Push通知連携

**目的:** インフラ管理者として、Firebase Cloud Messaging のサービスアカウントキーをシークレットとして登録したい。それにより、Go バックエンドが FCM HTTP API 経由で APNs に Push通知を送信できるようにする。

#### 受け入れ基準
1. The シークレット管理システム shall `FIREBASE_SERVICE_ACCOUNT_JSON` として Firebase サービスアカウントキー（JSON 形式）を Workers Secrets に登録する
2. When Workers Container が FCM HTTP API にリクエストを送信する, the Workers Container shall `FIREBASE_SERVICE_ACCOUNT_JSON` を使用して JWT 認証トークンを生成し送信する
3. The Cloudflare インフラ管理システム shall APNs との HTTP/2 接続管理・JWT 管理を直接行わず、FCM に委譲する
4. If FCM への送信が失敗した場合, the Workers Container shall エラーログを記録し呼び出し元に失敗を通知する
5. Where Firebase プロジェクトが Firebase コンソールで作成済みである, the シークレット管理システム shall サービスアカウントキーを Cloudflare に登録する前提とする

---

### 要件 7: GitHub Webhooks セキュリティ検証

**目的:** インフラ管理者として、GitHub Webhook の受信時に HMAC-SHA256 署名を検証するシークレットを登録したい。それにより、不正な Webhook リクエストを拒否できる安全な受信基盤を確立できるようにする。

#### 受け入れ基準
1. The シークレット管理システム shall `GITHUB_WEBHOOK_SECRET` を Workers Secrets に登録する
2. When Workers Container が GitHub Webhook リクエストを受信する, the Workers Container shall `X-Hub-Signature-256` ヘッダーを `GITHUB_WEBHOOK_SECRET` で HMAC-SHA256 検証する
3. If HMAC 署名の検証に失敗した場合, the Workers Container shall HTTP 403 レスポンスを返しリクエストを処理しない
4. The Cloudflare Workers shall GitHub Webhook の受信に必要な公開 URL を Workers Containers のデフォルト URL として提供する

---

### 要件 8: Terraform IaC 管理

**目的:** インフラ管理者として、Cloudflare リソース（Workers・D1・R2）を Terraform の `cloudflare` provider で管理したい。それにより、インフラ構成を再現可能なコードとして Git 管理できるようにする。

#### 受け入れ基準
1. The Terraform 構成 shall `cloudflare` provider を使用して D1・R2・Workers のリソースを定義する
2. When `terraform plan` が実行される, the Terraform shall 現在の state と差分を出力し適用前に確認できる
3. When `terraform apply` が実行される, the Terraform shall cloudflare provider を通じてリソースを作成・更新する
4. The Terraform 構成 shall シークレット値を `terraform.tfvars` や state に平文記録せず、Workers Secrets（Wrangler）で別途管理する
5. The Terraform state shall ローカルファイル（`terraform.tfstate`）として保存され、インフラ管理者 1 名が管理する（ハッカソンスコープ：Git 管理外、チーム共有なし）
6. If Terraform の apply 中にリソース作成エラーが発生した場合, the Terraform shall 影響範囲を特定できるエラーメッセージを表示する

---

### 要件 9: Wrangler デプロイ・DBマイグレーション

**目的:** インフラ管理者として、Wrangler を使って Workers・Workers Container のデプロイおよび D1 マイグレーションを実行したい。それにより、コードの変更を Cloudflare 環境に確実に反映できるようにする。

#### 受け入れ基準
1. The デプロイシステム shall 以下の順序でデプロイを実行できる: (1) Docker イメージビルド → (2) `wrangler deploy` → (3) `wrangler d1 migrations apply begit-db`
2. When `wrangler deploy` が実行される, the Wrangler shall Workers スクリプトと Workers Container イメージを Cloudflare に反映する
3. When `wrangler d1 migrations apply begit-db` が実行される, the Wrangler shall 未適用のマイグレーションのみを順次適用し、適用済みのものはスキップする
4. The ローカル開発環境 shall `wrangler dev` で Workers のローカル実行、`wrangler d1 execute` で D1 のローカル操作が可能である
5. If デプロイ中に Workers Container のプロビジョニングが未完了の場合, the Wrangler shall 初回プロビジョニングが完了するまで待機またはステータスを表示する
