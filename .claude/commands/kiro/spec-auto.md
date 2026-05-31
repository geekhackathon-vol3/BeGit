---
description: kiro SDD 全フェーズを自動実行（init→requirements→gap→design→tasks→impl）。--from で途中フェーズから再開可能。impl はサブエージェントで実行しコンテキストを分離。
allowed-tools: Bash, Read, Write, Edit, MultiEdit, Glob, Grep, LS, WebSearch, WebFetch, Agent
argument-hint: "<project-description>" | <feature-name> --from <phase>
---

# SDD Full Pipeline Auto Runner

引数: $ARGUMENTS

## 引数パターン

### A) 初回実行（全フェーズ）
```
/kiro:spec-auto "<project-description>"
```
- spec-init からすべてのフェーズを順番に実行する
- feature-name は spec-init が説明文から自動生成する

### B) 途中フェーズから再開
```
/kiro:spec-auto <feature-name> --from <phase>
```
- 指定した `<phase>` から実行を開始し、impl まで続ける
- `<phase>` に指定できる値:
  - `requirements` → Phase 2 から開始
  - `gap`          → Phase 3 から開始
  - `design`       → Phase 4 から開始
  - `tasks`        → Phase 5 から開始
  - `impl`         → Phase 6 のみ実行

## 引数の解析

$ARGUMENTS に `--from` が含まれているかを確認する:
- **含まれない** → モード A: 全文をプロジェクト説明として spec-init へ渡す
- **含まれる**   → モード B: `--from` の前をFEATURE_NAME、後ろを開始フェーズとして解析する

---

## 実行ルール

- **フェーズ間で承認・確認をユーザーに求めない**（完全自動）
- `-y` フラグ相当の自動承認を全フェーズに適用する
- エラーが発生した場合のみ停止してユーザーに報告する
- 各フェーズ完了を確認してから次へ進む
- モード B では開始フェーズより前のフェーズを**スキップ**する
- **各フェーズ完了後に必ずログファイルを書き込む**（後述）

---

## ログ書き込みルール

各フェーズ完了直後に、以下のパスへログファイルを Write する:

```
ai_log/riochin/FEATURE_NAME/<phase>.md
```

| フェーズ | ファイル名 |
|---------|-----------|
| init | `01_init.md` |
| requirements | `02_requirements.md` |
| gap | `03_gap.md` |
| design | `04_design.md` |
| tasks | `05_tasks.md` |
| impl（サブエージェント） | `06_impl.md` |

ログファイルのフォーマット:

```markdown
# [phase name] — FEATURE_NAME

**実行日時**: <ISO 8601 タイムスタンプ>
**フェーズ**: <フェーズ名>

## サマリー
<このフェーズで行ったことを 3〜5 箇条書き>

## 成果物
<作成・更新したファイルのパス一覧>

## 主要な決定事項 / 発見事項
<設計上の判断、ギャップ分析の結論、実装上の注意点など>

## 次フェーズへの引き継ぎ事項
<次のフェーズで注意すべき点や前提条件>
```

ログディレクトリが存在しない場合は Bash で `mkdir -p ai_log/riochin/FEATURE_NAME` を実行してから Write する。

---

## Phase 1: Spec Init
**モード A のみ実行。モード B では必ずスキップ。**

`.claude/commands/kiro/spec-init.md` を Read し、その指示に従って実行する。
- Arguments として渡す値: `$ARGUMENTS`（プロジェクト説明全文）

実行後:
- `Glob` で `.kiro/specs/*/spec.json` を取得し、新しく作成されたディレクトリ名を **FEATURE_NAME** として記録する
- 以降のすべてのフェーズでこの FEATURE_NAME を使用する
- `ai_log/riochin/FEATURE_NAME/01_init.md` を Write する

---

## Phase 2: Requirements Generation
**スキップ条件: `--from` の値が `gap` / `design` / `tasks` / `impl` の場合**

`.claude/commands/kiro/spec-requirements.md` を Read し、その指示に従って実行する。
- Arguments として渡す値: FEATURE_NAME
- 完了後: `ai_log/riochin/FEATURE_NAME/02_requirements.md` を Write する

---

## Phase 3: Gap Analysis
**スキップ条件: `--from` の値が `design` / `tasks` / `impl` の場合**

`.claude/commands/kiro/validate-gap.md` を Read し、その指示に従って実行する。
- Arguments として渡す値: FEATURE_NAME
- 完了後: `ai_log/riochin/FEATURE_NAME/03_gap.md` を Write する

---

## Phase 4: Design Generation
**スキップ条件: `--from` の値が `tasks` / `impl` の場合**

`.claude/commands/kiro/spec-design.md` を Read し、その指示に従って実行する。
- Arguments として渡す値: `FEATURE_NAME -y`
- `-y` フラグにより requirements を自動承認して即座に設計生成へ進む
- 完了後: `ai_log/riochin/FEATURE_NAME/04_design.md` を Write する

---

## Phase 5: Task Generation
**スキップ条件: `--from` の値が `impl` の場合**

`.claude/commands/kiro/spec-tasks.md` を Read し、その指示に従って実行する。
- Arguments として渡す値: `FEATURE_NAME -y`
- `-y` フラグにより requirements と design を自動承認して即座にタスク生成へ進む
- 完了後: `ai_log/riochin/FEATURE_NAME/05_tasks.md` を Write する

---

## Phase 6: Implementation（サブエージェント委譲）

**impl フェーズは計画フェーズで肥大化したコンテキストを引き継がないよう、必ずサブエージェントに委譲する。**

### 6-1. スペックファイルの読み込み

以下のファイルを Read して内容を取得する:
- `.kiro/specs/FEATURE_NAME/spec.json`
- `.kiro/specs/FEATURE_NAME/requirements.md`
- `.kiro/specs/FEATURE_NAME/design.md`
- `.kiro/specs/FEATURE_NAME/tasks.md`
- `.claude/commands/kiro/spec-impl.md`（impl コマンドの指示）

### 6-2. サブエージェントの起動

Agent ツールを使って実装専用のサブエージェントを起動する。
サブエージェントへの prompt は以下の構造で作成する:

```
あなたは kiro Spec-Driven Development の実装エージェントです。

## 対象フィーチャー
feature-name: FEATURE_NAME
作業ディレクトリ: <現在の working directory>

## 実行指示
以下のファイルを Read してから、spec-impl.md の指示に厳密に従って
tasks.md の全 `- [ ]` タスクを TDD サイクルで実装してください。

- `.kiro/specs/FEATURE_NAME/spec.json`
- `.kiro/specs/FEATURE_NAME/requirements.md`
- `.kiro/specs/FEATURE_NAME/design.md`
- `.kiro/specs/FEATURE_NAME/tasks.md`
- `.claude/commands/kiro/spec-impl.md`（実装手順の詳細）
- `.kiro/steering/` 以下の全ファイル（プロジェクトコンテキスト）

## 制約
- TDD 必須: テストを先に書いてから実装する
- タスク完了ごとに tasks.md の `- [ ]` を `- [x]` に更新する
- 実装は design.md の仕様に従う
- 承認を求めず全 pending タスクを完了させる
- **サブタスク（小数点単位: 1.1, 1.2, 2.1 ...）が完了するたびに git commit を作成する**
  - コミットメッセージ形式: `feat(FEATURE_NAME): task X.Y — <サブタスクの内容を一言で>`
  - tasks.md の `- [x]` 更新も同じコミットに含める
  - コミットは `git add` で関連ファイルのみステージングし、`git commit` で作成する

## ログ書き込み（必須）
実装完了後、以下のパスにログファイルを Write すること:
`ai_log/riochin/FEATURE_NAME/06_impl.md`

ログフォーマット:
---
# impl — FEATURE_NAME

**実行日時**: <ISO 8601 タイムスタンプ>
**フェーズ**: impl（サブエージェント）

## サマリー
<完了したタスクの概要を 3〜5 箇条書き>

## 成果物
<作成・更新したファイルのパス一覧>

## 主要な決定事項 / 発見事項
<実装中に判断したこと、設計との差異があれば記録>

## 次フェーズへの引き継ぎ事項
<残タスク・既知の問題・テスト結果など>
---
```

- Agent ツールの `description` には `"FEATURE_NAME impl phase"` を設定する
- `run_in_background: false` でサブエージェントの完了を待つ

### 6-3. 結果の受け取り

サブエージェントの完了後、その結果サマリーをユーザーに報告する。

---

## 完了時の出力

全フェーズ完了後、以下を出力する:

1. **実行フェーズ**: 実際に実行したフェーズの一覧（スキップしたフェーズも明示）
2. **ログファイル**: `ai_log/riochin/FEATURE_NAME/` に書き込んだファイル一覧
3. **作成/更新された Spec**: `.kiro/specs/FEATURE_NAME/` のファイル一覧
4. **実装サマリー**: サブエージェントが完了したタスク数と主要な変更ファイル
5. **次のステップ**: `/kiro:spec-status FEATURE_NAME` で最終確認
