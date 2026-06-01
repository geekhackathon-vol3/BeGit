# ギャップ分析: cloudflare-infra

**作成日:** 2026-05-31  
**フェーズ:** requirements-approved → design

---

## 分析サマリー

- **スコープ:** インフラ層は完全グリーンフィールド。`infra/` には設計ドキュメントのみ、`backend/` は `.gitkeep` だけで実装ゼロ
- **主な課題:** Workers Containers は比較的新しい機能（ベータ段階）で、Terraform cloudflare provider でのサポート状況・`wrangler.toml` の正確な構文を設計フェーズで要調査
- **推奨アプローチ:** Option B（新規コンポーネント作成）。既存インフラコードが存在しないため、拡張対象なし。新規ディレクトリ体系を設計から定義する
- **工数見積り:** M〜L（3〜10日）。Terraform + Wrangler + Docker の組み合わせだが Workers Containers の新規性がリスク要因
- **リスク:** Workers Containers の Terraform サポートと `wrangler.toml` binding 構文が中〜高リスク。他は低リスク

---

## 1. 現状調査

### 既存アセット

| パス | 内容 | 再利用可否 |
|------|------|-----------|
| `infra/infra.md` | アーキテクチャ設計書（テキスト） | 参照のみ |
| `infra/infra.mermaid` | 構成図 | 参照のみ |
| `backend/.gitkeep` | プレースホルダー | なし |
| `ios/` | SwiftUI アプリ（完全実装済み） | インフラ非関連 |

### 既存パターン・規約

| 観点 | 現状 |
|------|------|
| ディレクトリ規約 | `ios/` / `backend/` / `infra/` の 3 分割。モノレポ |
| CI/CD | `.github/workflows/labeler.yml`（PR ラベル付けのみ）。デプロイ CI なし |
| Git Hooks | `post-commit` で README 自動更新（`make setup` で有効化） |
| 命名規約 | Go: snake_case ファイル / PascalCase 型。インフラファイルへの規約記述なし |

### 統合サーフェス（iOS との接続点）

iOS の `Services/AuthAPI.swift` がバックエンド API エンドポイントを呼び出す構造になっているため、Workers Container のデフォルト URL が iOS 側設定と一致する必要がある。ただし Workers Containers はデフォルトでパブリック URL が付与されるため、別途ドメイン管理は不要。

---

## 2. 要件フィージビリティ分析

### 要件ごとの技術ニーズとギャップ

#### 要件 1: Cloudflare Workers エントリーポイント

| 技術ニーズ | 現状 | ギャップ |
|-----------|------|---------|
| `wrangler.toml` 作成 | 存在しない | **Missing** — ゼロから作成 |
| D1 binding 定義 | なし | **Missing** |
| R2 binding 定義 | なし | **Missing** |
| Workers Containers binding | なし | **Missing** + **Research Needed**（`containers` binding の `wrangler.toml` 構文）|
| Workers スクリプト（ルーティング JS/TS） | なし | **Missing** — fetch proxy スクリプト要作成 |

#### 要件 2: Workers Containers (Go API) デプロイ

| 技術ニーズ | 現状 | ギャップ |
|-----------|------|---------|
| Dockerfile (linux/amd64) | 存在しない | **Missing** — `backend/Dockerfile` 作成 |
| Go バックエンドコード | なし（`.gitkeep` のみ） | **Missing** — ただし本 spec のスコープ外（backend API 実装は別 spec） |
| `sleepAfter` 設定 | なし | **Missing** + **Research Needed**（`wrangler.toml` の正確な設定キー）|
| Cloudflare コンテナレジストリへの push | なし | **Missing** + **Research Needed**（`wrangler deploy` が自動管理するか手動 push か）|

> **注:** Workers Container が実際に実行する Go コードは本 spec の対象外。Dockerfile と wrangler.toml の設定が主な成果物。

#### 要件 3: Cloudflare D1 データベース管理

| 技術ニーズ | 現状 | ギャップ |
|-----------|------|---------|
| Terraform `cloudflare_d1_database` リソース | なし | **Missing** |
| `begit-db` D1 リソース | なし | **Missing** |
| マイグレーションディレクトリ (`migrations/`) | なし | **Missing** — SQLite 方言で作成 |
| `wrangler d1 migrations apply` 手順 | 文書化済み（infra.md） | 実装なし |

#### 要件 4: Cloudflare R2 写真ストレージ設定

| 技術ニーズ | 現状 | ギャップ |
|-----------|------|---------|
| Terraform `cloudflare_r2_bucket` リソース | なし | **Missing** |
| `begit-photos` バケット | なし | **Missing** |
| S3 互換 API アクセス設定 | なし | **Research Needed**（CORS・パブリックアクセス設定の要否）|

#### 要件 5 / 6 / 7: Workers Secrets シークレット管理

| 技術ニーズ | 現状 | ギャップ |
|-----------|------|---------|
| `wrangler secret put` 手順書 | なし | **Missing** — Makefile ターゲット or README に手順記載 |
| Terraform での secrets 管理回避 | 要設計 | **Constraint** — `terraform.tfvars` に平文記録しない設計が必要 |
| Firebase サービスアカウント JSON 取得手順 | なし | **Missing** — Firebase コンソール操作（本 spec スコープ外だが前提手順の文書化が必要）|
| HMAC-SHA256 検証ロジック | なし（backend 未実装）| **Missing** — backend API 実装 spec でカバー |

#### 要件 8: Terraform IaC 管理

| 技術ニーズ | 現状 | ギャップ |
|-----------|------|---------|
| `infra/terraform/` ディレクトリ | なし | **Missing** — ゼロから作成 |
| `cloudflare` provider 設定 | なし | **Missing** + **Research Needed**（最新 provider バージョンとリソース名）|
| Terraform state 管理 | なし | **Research Needed** — ローカル or Cloudflare R2 backend or Terraform Cloud |
| Workers リソースの Terraform サポート | 不明 | **Research Needed** — Workers Containers の Terraform resource が存在するか |

#### 要件 9: Wrangler デプロイ・DBマイグレーション

| 技術ニーズ | 現状 | ギャップ |
|-----------|------|---------|
| デプロイ手順スクリプト/Makefile | `Makefile` に `setup` のみ | **Missing** — `deploy` ターゲット追加 |
| `wrangler dev` ローカル実行設定 | なし | **Missing** — `wrangler.toml` 作成で解決 |
| マイグレーション適用手順 | 文書化済み | 実装なし |

---

## 3. 実装アプローチの選択肢

### Option A: 既存コンポーネントの拡張

**適用不可。** インフラコードが存在しないため拡張対象なし。

---

### Option B: 新規コンポーネント作成（推奨）

**理由:** グリーンフィールドのため新規ディレクトリ体系を一から設計できる。責務が明確に分離される。

**作成するファイル/ディレクトリ:**

```
infra/
  terraform/
    main.tf           # provider 設定・terraform backend
    variables.tf      # API token 等の変数定義
    outputs.tf        # D1 ID・R2 バケット名等の出力
    d1.tf             # cloudflare_d1_database リソース
    r2.tf             # cloudflare_r2_bucket リソース
    workers.tf        # Workers 関連リソース（存在すれば）
    terraform.tfvars.example  # シークレット除外のサンプル

backend/
  wrangler.toml       # Workers + Container + D1/R2 binding 設定
  Dockerfile          # linux/amd64 Go API イメージ
  migrations/
    0001_initial.sql  # D1 初期スキーマ（SQLite 方言）
```

**Makefile への追記:**
```makefile
deploy:         ## Docker build → wrangler deploy → D1 migration
secrets-init:   ## wrangler secret put の一括実行手順
```

**トレードオフ:**
- ✅ クリーンな責務分離（Terraform vs Wrangler）
- ✅ 各ファイルを独立してテスト・適用可能
- ✅ 既存 iOS コードへの影響ゼロ
- ❌ Workers Containers の Terraform サポートが未確認（設計フェーズで調査必須）
- ❌ 新規ディレクトリ体系のため命名規約を新たに決める必要あり

---

### Option C: ハイブリッドアプローチ

**条件付き考慮。** Workers Containers が Terraform でサポートされない場合、以下のハイブリッドが現実的:

- **Terraform 管理:** D1・R2 のリソース作成のみ
- **Wrangler 管理:** Workers デプロイ・Container プロビジョニング・DB マイグレーション・シークレット

この分担は `infra.md` が既に示している構成と一致しており、Option B の推奨実装と実質同じになる可能性が高い。

---

## 4. 調査が必要な項目（設計フェーズへ持ち越し）

| # | 調査項目 | 理由 | 優先度 |
|---|---------|------|--------|
| R1 | Terraform cloudflare provider の Workers Containers サポート状況 | `cloudflare_worker_script` vs 専用リソースの有無 | 高 |
| R2 | `wrangler.toml` の `containers` binding 正確な構文 | Workers Containers は新機能でドキュメントが変化中 | 高 |
| R3 | `sleepAfter` の設定キー名と単位 | wrangler.toml に記述するか container 設定ファイルか | 中 |
| R4 | Terraform state backend の選択 | ローカル管理 vs R2 backend vs Terraform Cloud | 中 |
| R5 | R2 バケットの CORS 設定・パブリックアクセス要否 | iOS クライアントが直接 R2 に読み書きするか Workers 経由か | 中 |
| R6 | Workers エントリーポイント JS の最小実装 | fetch proxy のみか ESM export が必要か | 低 |

---

## 5. 複雑度・リスク評価

| コンポーネント | 工数 | リスク | 根拠 |
|--------------|------|--------|------|
| Terraform (D1 / R2) | S | 低 | cloudflare provider に既存リソース定義あり、パターン明確 |
| Terraform (Workers) | M | 中 | Workers Containers の Terraform サポートが未確認 |
| `wrangler.toml` + Workers スクリプト | M | 中〜高 | Workers Containers binding 構文が新機能で変化中 |
| Dockerfile (linux/amd64) | S | 低 | Go の標準 multi-stage build、アーキテクチャ指定も単純 |
| D1 マイグレーション | S | 低 | SQLite 方言・wrangler コマンドともに確立された手順 |
| シークレット管理手順 | S | 低 | `wrangler secret put` は単純なコマンド |
| Makefile / デプロイスクリプト | S | 低 | 既存 Makefile への追記 |
| **全体** | **M〜L** | **中** | Workers Containers の新規性が全体リスクを押し上げる |

---

## 6. 設計フェーズへの推奨事項

### 推奨アプローチ
**Option B（新規コンポーネント作成）を採用し、Terraform と Wrangler の責務を明確に分担する。**

具体的な分担:
- **Terraform:** D1 データベース / R2 バケットのリソース作成
- **Wrangler:** Workers デプロイ / Container プロビジョニング / シークレット管理 / DB マイグレーション

### 設計フェーズで確定すべき重要決定事項

1. **R1 調査:** cloudflare provider で Workers をどこまで Terraform 管理するか → Workers Containers を Terraform に含めるかは調査結果次第
2. **R2 調査:** `wrangler.toml` の Workers Containers binding 構文の確定
3. **Terraform state 管理方法の決定:** ハッカソンスコープでは `local` backend で可能だが、チーム共有を考慮すると R2 backend も検討
4. **ディレクトリ構造の確定:** `infra/terraform/` 内のファイル分割方針
5. **D1 スキーマ設計との連携:** backend API 実装 spec と D1 マイグレーションの設計を合わせる必要あり
