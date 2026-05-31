# 実装タスクリスト: cloudflare-infra

## 意図的に除外した要件

設計書で「別 spec（Go バックエンド実装）または手動作業スコープ外」として明示されているため除外する:

| 要件 ID | 内容 | 除外理由 |
|---------|------|----------|
| 6.2 | FCM HTTP API への JWT 認証リクエスト | Go バックエンド実装 spec でカバー |
| 6.3 | APNs との直接接続を FCM に委譲 | アーキテクチャ方針（設計済み） |
| 6.4 | FCM 失敗時のエラーログ・通知 | Go バックエンド実装 spec でカバー |
| 6.5 | Firebase プロジェクト作成済みを前提 | 手動作業（Firebase コンソール）・スコープ外 |
| 7.2 | HMAC-SHA256 署名検証 | Go バックエンド実装 spec でカバー |
| 7.3 | 署名検証失敗時の 403 レスポンス | Go バックエンド実装 spec でカバー |

---

- [ ] 1. Terraform IaC 基盤を構築する

- [ ] 1.1 Terraform プロジェクト構造と provider 設定を作成する
  - `infra/terraform/` ディレクトリを作成し、`main.tf`・`variables.tf` を追加する
  - `main.tf`: cloudflare provider `~> 4` を `required_providers` に指定し、local backend を設定する
  - `variables.tf`: `cloudflare_account_id`（必須）と `cloudflare_api_token`（`TF_VAR_` 環境変数経由）を定義する
  - `terraform.tfvars.example`: `cloudflare_account_id` のみ記載したサンプルファイルを作成し、シークレット値は含めない
  - `.gitignore` に `terraform.tfstate`・`terraform.tfstate.backup`・`terraform.tfvars` を追加する
  - `terraform init` が正常に完了し `.terraform/` ディレクトリが生成される
  - _Requirements: 8.1, 8.4, 8.5_

- [ ] 1.2 D1 データベースと R2 バケットの Terraform リソースを定義する
  - `d1.tf`: `cloudflare_d1_database "begit_db"` リソース（`name = "begit-db"`）を定義する
  - `r2.tf`: `cloudflare_r2_bucket "begit_photos"` リソース（`name = "begit-photos"`）を定義する
  - `outputs.tf`: `d1_database_id`（D1 データベース ID）と `r2_bucket_name` を出力定義する
  - apply 失敗時は Terraform がエラー内容を標準出力に表示し、部分適用済みリソースを state ファイルに記録する（built-in 動作）
  - `terraform plan` を実行すると `begit-db` D1 リソースと `begit-photos` R2 バケットが作成対象として表示される
  - _Requirements: 1.2, 1.4, 3.1, 4.1, 4.4, 8.2, 8.6_

- [ ] 2. wrangler.toml と依存パッケージを設定する
  - `backend/package.json` を作成し、`wrangler` と `@cloudflare/containers` を devDependencies に追加する
  - `backend/wrangler.toml` を作成し、`name`・`main = "src/index.ts"`・`compatibility_date` を設定する
  - `[[containers]]`: `class_name = "BeGitAPI"`・`image = "./Dockerfile"`・`max_instances = 1` を定義する
  - `[[durable_objects.bindings]]`: `name = "BEGIT_API"`・`class_name = "BeGitAPI"` を定義する
  - `[[migrations]]`: `tag = "v1"`・`new_classes = ["BeGitAPI"]` を定義する
  - `[[d1_databases]]`: `binding = "DB"`・`database_name = "begit-db"`・`database_id = "<REPLACE_WITH_TERRAFORM_OUTPUT>"` を定義する
  - `[[r2_buckets]]`: `binding = "PHOTOS"`・`bucket_name = "begit-photos"` を定義する
  - `vars` セクションにシークレット値を平文で記載しない（シークレットは Workers Secrets で管理）
  - `wrangler dev` コマンドが起動エラーなく実行できる（ローカル開発環境が起動する）
  - _Requirements: 1.1, 2.4, 3.5, 4.2, 4.3, 5.2, 9.2, 9.4_

- [ ] 3. コアコンポーネントを実装する

- [ ] 3.1 (P) Workers TypeScript エントリーポイントを実装する
  - `backend/src/index.ts` を作成する
  - `BeGitAPI extends Container` クラスを定義: `defaultPort = 8080`・`sleepAfter = "10m"`
  - `Env` インターフェースを定義: `BEGIT_API`（DurableObjectNamespace）・`DB`（D1Database）・`PHOTOS`（R2Bucket）・4 シークレット（文字列型）
  - `export default { fetch }` ハンドラを実装: `getContainer(env.BEGIT_API, env.BEGIT_API.idFromName("begit-api-singleton")).fetch(request)` でシングルトン DO に転送する
  - `wrangler dev` でローカル Workers が起動し、リクエストが Container に転送される
  - _Requirements: 1.3, 1.5, 2.3, 4.5, 5.4, 7.4_
  - _Boundary: Workers Entry Point_

- [ ] 3.2 (P) Go API の linux/amd64 Dockerfile を作成する
  - `backend/Dockerfile` を作成し、`--platform=linux/amd64` 指定の multi-stage ビルドを定義する
  - Stage 1（ビルダー）: `golang:alpine` ベースで Go ソースをコンパイルする
  - Stage 2（ランタイム）: `alpine` ベース、`adduser -D appuser` で非 root ユーザーを作成し `USER appuser` で実行する
  - `EXPOSE 8080`（Workers Entry Point の `defaultPort = 8080` と一致させる）
  - `docker build --platform linux/amd64 -t begit-api .` が exit code 0 で完了する
  - _Requirements: 2.1, 2.5_
  - _Boundary: Dockerfile_

- [ ] 3.3 (P) D1 マイグレーション初期 SQL ファイルを作成する
  - `backend/migrations/0001_initial.sql` を作成する
  - SQLite 方言のプレースホルダースキーマを記述する（`INTEGER PRIMARY KEY AUTOINCREMENT`・`TEXT`・`INTEGER` のみ使用）
  - `SERIAL`・`BOOLEAN`・`DATETIME` 等 PostgreSQL 固有の型は使用しない
  - `wrangler d1 migrations apply begit-db --local` で SQL 文法エラーなく適用が完了する
  - _Requirements: 3.2, 3.3, 3.4, 9.3_
  - _Boundary: D1 Migrations_

- [ ] 4. Makefile デプロイ自動化を実装する

- [ ] 4.1 terraform-apply ターゲットを Makefile に追加する
  - 既存 `Makefile` に `terraform-apply` ターゲットを追記する
  - `terraform -chdir=infra/terraform apply` を実行する
  - apply 完了後、`terraform -chdir=infra/terraform output -raw d1_database_id` で D1 ID を取得する
  - `sed -i` で `backend/wrangler.toml` の `database_id` プレースホルダーを実際の ID に置換する
  - `make terraform-apply` 実行後、`backend/wrangler.toml` の `database_id` が実際の Cloudflare D1 ID で更新されている
  - _Requirements: 8.3_

- [ ] 4.2 deploy ターゲットを Makefile に追加する
  - 既存 `Makefile` に `deploy` ターゲットを追記する
  - `docker build --platform linux/amd64 -t begit-api ./backend` → `cd backend && wrangler deploy` → `wrangler d1 migrations apply begit-db` の順で実行する
  - Makefile の `&&` チェーンにより各ステップが exit code 非ゼロの場合に後続を中断する
  - `make deploy` の実行でイメージビルド・Workers デプロイ・DB マイグレーションの全工程が完了する
  - _Requirements: 2.2, 9.1, 9.2, 9.5_

- [ ] 4.3 secrets-init と warmup ターゲットを Makefile に追加する
  - 既存 `Makefile` に `secrets-init` ターゲットを追記する
  - `GITHUB_CLIENT_SECRET`・`GITHUB_WEBHOOK_SECRET`・`FIREBASE_SERVICE_ACCOUNT_JSON`・`DB_ENCRYPTION_KEY` の `wrangler secret put` コマンド手順を `@echo` で表示する
  - シークレット値はスクリプト内に記載しない（管理者が各コマンドを手動実行するガイドのみ出力）
  - `warmup` ターゲットを追加: Workers デフォルト URL に `curl` でリクエストを送信しコンテナをウォームアップする
  - `make secrets-init` 実行で 4 シークレットの登録コマンド手順が標準出力に表示される
  - _Requirements: 5.1, 5.3, 5.5, 6.1, 7.1_

- [ ] 5. インフラ動作確認

- [ ] 5.1 ローカル開発環境での接続確認を行う
  - `wrangler d1 execute begit-db --local --command "SELECT 1"` でローカル D1 接続が成功することを確認する
  - `wrangler dev` でローカル Workers が `http://localhost:8787` で起動し、リクエストに応答することを確認する
  - _Requirements: 9.4_

- [ ] 5.2 デプロイ後のスモークテストを実行する
  - `wrangler secret list` で `GITHUB_CLIENT_SECRET`・`GITHUB_WEBHOOK_SECRET`・`FIREBASE_SERVICE_ACCOUNT_JSON`・`DB_ENCRYPTION_KEY` の 4 シークレットが登録済みであることを確認する
  - Workers デプロイ後に `curl https://<workers-url>/` でレスポンスが返ることを確認する
  - `wrangler d1 execute begit-db --command "SELECT name FROM sqlite_master WHERE type='table'"` でマイグレーション適用済みテーブルが存在することを確認する
  - `terraform -chdir=infra/terraform plan` で差分ゼロ（リソース変更なし）であることを確認する
  - _Requirements: 5.1, 9.2_
