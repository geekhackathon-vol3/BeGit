# Research & Design Decisions

---
**Purpose**: Cloudflare インフラ設計のディスカバリー調査記録

---

## Summary
- **Feature**: `cloudflare-infra`
- **Discovery Scope**: New Feature（グリーンフィールド）/ Complex Integration
- **Key Findings**:
  - Workers Containers は Durable Objects ベースのアーキテクチャ。`sleepAfter` は `wrangler.toml` でなく TypeScript の Container クラス内プロパティとして定義する
  - Terraform cloudflare provider は Workers Containers をサポートしない。D1（`cloudflare_d1_database`）・R2（`cloudflare_r2_bucket`）のリソース作成は Terraform、Workers/Container のデプロイは Wrangler という役割分担が公式推奨
  - Terraform state は Cloudflare R2 を S3 互換バックエンドとして利用可能だが、ハッカソンスコープでは local backend が現実的

---

## Research Log

### Topic 1: Workers Containers のアーキテクチャと wrangler.toml 構文

- **Context**: gap-analysis で「Workers Containers binding 構文が要調査（高優先）」と記録
- **Sources Consulted**:
  - [Cloudflare Containers Getting Started](https://developers.cloudflare.com/containers/get-started/)
  - [Cloudflare Containers Overview](https://developers.cloudflare.com/containers/)
  - [Community: sleepAfter and container termination](https://community.cloudflare.com/t/cloudflare-containers-help-understanding-sleepafter-and-container-termination/863468)
- **Findings**:
  - Workers Containers は内部的に Durable Objects として実装される
  - `wrangler.toml` に必要なセクション:
    ```toml
    [[containers]]
    class_name = "BeGitAPI"
    image = "./Dockerfile"
    max_instances = 1

    [[durable_objects.bindings]]
    name = "BEGIT_API"
    class_name = "BeGitAPI"

    [[migrations]]
    tag = "v1"
    new_sqlite_classes = ["BeGitAPI"]
    ```
  - Worker TypeScript スクリプトにて Container クラスを定義し、`sleepAfter`・`defaultPort` を設定:
    ```typescript
    export class BeGitAPI extends Container {
      defaultPort = 8080;
      sleepAfter = "10m";
    }
    ```
  - ルーティングは Worker の `fetch` ハンドラで `getContainer(env.BEGIT_API, id).fetch(request)` により実現
  - `new_sqlite_classes` が必須（`new_classes` ではない）
- **Implications**: Worker TypeScript ファイルが必要。単純な fetch proxy だが、Durable Object ID の戦略（シングルトン vs リクエストキー別）を設計で決定する必要がある

### Topic 2: Terraform cloudflare provider の Workers Containers サポート状況

- **Context**: gap-analysis で「R1: Workers Containers の Terraform サポート（高優先）」
- **Sources Consulted**:
  - [Infrastructure as Code · Cloudflare Workers docs](https://developers.cloudflare.com/workers/platform/infrastructure-as-code/)
  - [Terraform Registry: cloudflare_worker](https://registry.terraform.io/providers/cloudflare/cloudflare/latest/docs/resources/worker)
  - [Deploy Workers using Terraform](https://blog.cloudflare.com/deploy-workers-using-terraform/)
- **Findings**:
  - Workers Containers は Terraform cloudflare provider でサポートされない（2026-05 時点）
  - 標準 Workers (`cloudflare_worker`, `cloudflare_worker_version`) は Terraform 管理可能
  - 公式ドキュメントでも Workers Containers のデプロイは Wrangler を推奨
- **Implications**: Workers・Container のデプロイは Wrangler 専任。Terraform は D1・R2 のリソース作成のみに限定する

### Topic 3: Terraform D1・R2 リソース名と構文

- **Context**: Terraform で実際に使用するリソース名の確認
- **Sources Consulted**:
  - [Terraform Registry: cloudflare_r2_bucket](https://registry.terraform.io/providers/cloudflare/cloudflare/latest/docs/resources/r2_bucket)
  - [Terraform Registry: cloudflare_d1_database](https://registry.terraform.io/providers/cloudflare/cloudflare/latest/docs/resources/d1_database)
  - [Terraform · Cloudflare R2 docs](https://developers.cloudflare.com/r2/examples/terraform/)
- **Findings**:
  - R2 バケット: `cloudflare_r2_bucket` リソース。必須引数: `account_id`, `name`。オプション: `location`
  - D1 データベース: `cloudflare_d1_database` リソース
  - provider バージョン `~> 4` で両リソース利用可能
  - R2 の CORS・ライフサイクル設定は `cloudflare_r2_bucket_lifecycle` で別途管理（本 spec では CORS 設定不要: Workers 経由アクセスのみ）
- **Implications**: Terraform ファイル構成が確定。`cloudflare_d1_database.begit_db.id` を output して wrangler.toml の `database_id` に設定する連携が必要

### Topic 4: Terraform state のリモートバックエンド（R2）

- **Context**: チーム共有・ハッカソンスコープでの state 管理方針
- **Sources Consulted**:
  - [Remote R2 backend · Cloudflare Terraform docs](https://developers.cloudflare.com/terraform/advanced-topics/remote-backend/)
- **Findings**:
  - R2 は S3 互換 API を持つため、`backend "s3"` で R2 を Terraform state バックエンドとして利用可能
  - 設定例:
    ```hcl
    backend "s3" {
      bucket = "terraform-state"
      key    = "begit/terraform.tfstate"
      region = "auto"
      skip_credentials_validation = true
      skip_metadata_api_check     = true
      skip_region_validation      = true
      skip_requesting_account_id  = true
      skip_s3_checksum            = true
      use_path_style              = true
      endpoints = { s3 = "https://<ACCOUNT_ID>.r2.cloudflarestorage.com" }
    }
    ```
- **Implications**: ハッカソンスコープでは `local` backend で十分だが、チーム開発では R2 backend を検討する価値あり。本 spec では `local` を採用し、R2 backend の設定例を `terraform.tfvars.example` にコメントで残す

---

## Architecture Pattern Evaluation

| Option | Description | Strengths | Risks / Limitations | Notes |
|--------|-------------|-----------|---------------------|-------|
| A: Terraform のみ | D1・R2・Workers すべてを Terraform で管理 | 統一 IaC ツール | Workers Containers が Terraform 非対応 | 採用不可 |
| B: Wrangler のみ | すべてを Wrangler で管理 | ツール統一 | D1・R2 の宣言的リソース管理が弱い | 要件 8 の IaC 要件を満たせない |
| C: Terraform (D1・R2) + Wrangler (Workers・Secrets・Migration) | 公式推奨の役割分担 | 各ツールの得意領域を活用。公式ドキュメントが整備 | ツールが 2 つになる複雑さ | **採用** |

---

## Design Decisions

### Decision: Workers Container の DO ID 戦略

- **Context**: Durable Object として実装される Container インスタンスを、どの ID で取得するか
- **Alternatives Considered**:
  1. シングルトン（固定名 ID）— 全リクエストを 1 インスタンスに集約
  2. リクエストキー別（URL・セッション ID 別）— リクエストごとに別インスタンス
- **Selected Approach**: シングルトン（`idFromName("begit-api-singleton")`）
- **Rationale**: ハッカソンスコープでは 1 インスタンスで十分。スケールアウトが必要な場合は URL パスベースに変更可能
- **Trade-offs**: スケールアウト不可（`max_instances = 1`）だがシンプル。デモ用途には最適
- **Follow-up**: 本番運用時はリクエストキー別 DO ID 戦略への移行を検討

### Decision: Terraform state backend

- **Context**: IaC の state をどこで管理するか
- **Alternatives Considered**:
  1. Local backend — 簡単だがチーム共有困難
  2. R2 S3-compatible backend — チーム共有可能
  3. Terraform Cloud — 外部サービス追加
- **Selected Approach**: Local backend（ハッカソン期間）、R2 backend 設定例をコメントで残す
- **Rationale**: ハッカソン（短期・少人数）では local で十分。R2 backend の設定手順を残しておくことで後続の本番化が容易
- **Trade-offs**: チーム間で state ファイルの共有が手動になる

### Decision: Workers エントリーポイントの言語

- **Context**: Worker スクリプト（ルーティング）を TypeScript で書くか JavaScript で書くか
- **Alternatives Considered**:
  1. TypeScript — 型安全、`@cloudflare/workers-types` で補完
  2. JavaScript — 設定不要でシンプル
- **Selected Approach**: TypeScript（`src/index.ts`）
- **Rationale**: `Container` クラス定義やバインディング型安全性のために TypeScript が適切。`@cloudflare/containers` パッケージが TypeScript 前提

### Decision: D1 ID を wrangler.toml に渡す方法

- **Context**: Terraform で作成した D1 の `database_id` を `wrangler.toml` に設定する方法
- **Alternatives Considered**:
  1. `terraform output` で取得し手動で `wrangler.toml` に記述
  2. Makefile で `terraform output -raw d1_database_id` を実行し wrangler.toml を自動更新
  3. `wrangler.toml` に `${D1_DATABASE_ID}` 変数を使用（wrangler 変数展開）
- **Selected Approach**: `terraform output` → Makefile で手順化し、初回セットアップ手順として文書化
- **Rationale**: wrangler.toml の変数展開は environment variable で `wrangler.toml` を書き換えるより、Makefile に `terraform-output` ターゲットを設け出力を表示する方がシンプル
- **Follow-up**: 実装時に Makefile ターゲット設計で最終決定

---

## Risks & Mitigations

- Workers Containers (beta) のAPI変更リスク — `wrangler.toml` の `containers` セクション構文は beta 段階。`wrangler` を最新版に固定し、`package.json` の `devDependencies` でバージョンを管理することで影響を限定
- コールドスタート（`sleepAfter` 後） — `sleepAfter = "10m"` で一定期間ウォームに保つ。デモ前にウォームアップリクエストを送るオペレーション手順を Makefile に追加
- Terraform state のチーム共有 — ハッカソン期間は local で許容。本番前に R2 backend への移行手順を残す
- D1 スキーマと backend API 実装の非同期 — D1 マイグレーション SQL は backend API 実装 spec と同期して作成する。本 spec では初期スキーマ（テーブル定義プレースホルダー）のみ作成

---

## References

- [Cloudflare Containers Getting Started](https://developers.cloudflare.com/containers/get-started/)
- [Cloudflare Containers Overview](https://developers.cloudflare.com/containers/)
- [Infrastructure as Code · Cloudflare Workers docs](https://developers.cloudflare.com/workers/platform/infrastructure-as-code/)
- [Terraform · Cloudflare R2 docs](https://developers.cloudflare.com/r2/examples/terraform/)
- [Remote R2 backend · Cloudflare Terraform docs](https://developers.cloudflare.com/terraform/advanced-topics/remote-backend/)
- [cloudflare_r2_bucket | Terraform Registry](https://registry.terraform.io/providers/cloudflare/cloudflare/latest/docs/resources/r2_bucket)
- [cloudflare_d1_database | Terraform Registry](https://registry.terraform.io/providers/cloudflare/cloudflare/latest/docs/resources/d1_database)
- [Community: sleepAfter and container termination](https://community.cloudflare.com/t/cloudflare-containers-help-understanding-sleepafter-and-container-termination/863468)
