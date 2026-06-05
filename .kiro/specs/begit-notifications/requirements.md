# Requirements Document

## Project Description (Input)

BeGit; の通知機能の**バックエンド実装**を仕様化する。全体設計（通知カタログ・設計判断の理由）は [`docs/notification/design.md`](../../../docs/notification/design.md) (v0.3.0) を、iOS 側の契約は [`docs/notification/ios-guide.md`](../../../docs/notification/ios-guide.md) を土台とする。本 spec はそのうち **バックエンド（Go / Cloudflare Workers Containers + D1 + R2 + FCM）が担う範囲のみ** を対象とする。

通知は3カテゴリ7種類。バックエンドはこれらを発火条件に応じて検知・生成し、FCM 経由で配信し、関連データを D1 に永続化する。

### 対象（バックエンドのスコープ）

【A. チャレンジ系】
- ① BeGit Time! 発行 API（既存 begit-backend-api Req3、`POST /groups/:id/notifications`、グループ全員へ FCM 送信、1スプリント1人1回 `UNIQUE(sprint_id,sent_by)` 維持）。**追加ルール**: 同一スプリント内でアクティブな(発行+1h以内)チャレンジが存在する間は新規発行不可をサービス層で担保し 409 を返す（時間的非共存 → ② の anchor が一意になる）。
- ② Nice Work!（GitHub Webhook 検知 → 行動した本人へ FCM 送信）。トリガーは commit(push) / issue(issues opened) / review(pull_request_review) の3種。検知窓は発行後〜スプリント終了まで、Late でも発火。1チャレンジ1人1回（`posts.UNIQUE(notification_id,user_id)` で冪等）。anchor = 検知時刻以前で最新の BeGit Time 通知。検知データ(commit数/additions/deletions/repo/branch/最新コミットメッセージ)を `posts` の draft 状態として保存（写真有無では判定せず明示 `status='draft'`/`is_draft`）。`sent_at + 1h` と比較し on_time/late を確定。
- ③ チャレンジ終了通知（Cron、発行+1h、グループ全員へ結果サマリ On Time/Late/Missed 集計）。

【B. スプリント系（Cron 駆動、グループ全員へ FCM 送信）】
- ④ スプリント終了3日前リマインダー（`ends_at − 3日`）。
- ⑤ スプリント終了通知（`ends_at` 到達、結果まとめ）。
- ⑥ 新スプリント開始通知（次スプリント INSERT 時）。

【C. ソーシャル系】
- ⑦ リアクション/コメント通知（投稿への反応を検知 → 投稿者本人へ FCM 送信、自己通知抑制）。

### バックエンド差分（実装が必要な変更）
- Webhook 登録イベントに `issues` を追加（現状 push / pull_request_review のみ）。
- Webhook ハンドラへ ② 発火ロジック追加（メンバー判定 → anchor 特定 → 初アクティビティ判定 → 下書き保存 → on_time/late 判定 → 本人へ FCM）。
- ① の時間非共存ルール（サービス層）。
- `posts` へ draft 状態を追加しフィード API で除外（D1 マイグレーション）。
- Cron へ ③④⑤⑥ のトリガー追加 + 結果サマリ算出。
- FCM ペイロードの `type` 拡張（begit_time / nice_work / challenge_end / sprint_reminder / sprint_end / sprint_start / reaction / comment）。契約フィールドは ios-guide.md §2 を満たす。

### Out of Scope（iOS 担当 / 契約は ios-guide.md 参照）
- `AppDelegate` の `type` 別画面遷移。
- ① BeGit Time タップ → memo タイプ投稿作成 UI。
- ② Nice Work タップ → 写真撮影 → 下書きプレフィル → 投稿確定の画面フロー。
- On Time / Late バッジ表示。
- バックエンドは上記 UI が依存する **FCM ペイロード契約** と **API（OpenAPI）** を提供する責務のみを負う。

### 関連既存 spec
- [`begit-backend-api`](../begit-backend-api)（Req3 通知発行 / Req5 Webhook 受信 / Req6 FCM トークン管理）を参照・一部更新する。

---

## Introduction

本要件は BeGit; の通知バックエンドが満たすべき振る舞いを EARS 形式で定義する。対象システムは Cloudflare Workers をエントリーポイントとし Workers Containers 上で動作する Go サーバー（以下「BeGit 通知バックエンド」）である。通知の検知・生成・配信・永続化のすべてをこのバックエンドが担い、iOS クライアントとは **FCM ペイロード契約**（[ios-guide.md](../../../docs/notification/ios-guide.md) §2）と **REST API 契約**（OpenAPI）で接続する。

要件は「WHAT（何を満たすか）」に集中し、実装手段（HOW）は設計フェーズへ委ねる。design.md §9 に挙がる未決論点は本書末尾の「設計フェーズへの申し送り事項」に集約し、要件で前提を確定できるものは確定し、確定できないものは設計フェーズ送りとして明示する。

## Boundary Context

- **In scope**: ①〜⑦ 通知の発火条件判定・データ永続化・FCM 送信（data ペイロード契約準拠）、BeGit Time! の時間的非共存ルール、Webhook の `issues` イベント追加と ② 発火ロジック、`posts` の draft 状態と feed 除外、Cron 基盤（③④⑤⑥）と結果サマリ算出、Webhook/Cron の冪等性。
- **Out of scope**: iOS の画面遷移・撮影・下書きプレフィル UI・バッジ表示（上記「Out of Scope」参照）。Cloudflare のルーティング/Cron スケジュール定義そのものの IaC 管理（Terraform/Wrangler 設定値の運用）は本 spec の関心外だが、Cron トリガーが必要である事実は In scope として明記する。
- **Adjacent expectations**: begit-backend-api の既存スキーマ（`0001_initial.sql`）・既存エンドポイント（Req3/5/6）・FCM サービスアカウント設定・GitHub OAuth App 設定が存在する前提。iOS は ios-guide.md §2 の `type`/フィールドに従って分岐する。

---

## Requirements

### Requirement 1: BeGit Time! 発行と時間的非共存ルール

**Objective:** As a グループメンバー, I want 同時に1つだけアクティブになるよう BeGit Time! を発行できる, so that 各チャレンジの応答（② Nice Work!）の起点（anchor）が一意に定まる

#### Acceptance Criteria

1. When 認証済みグループメンバーが `POST /groups/:id/notifications` を呼んだ, the BeGit 通知バックエンドは当日スプリントを取得または作成し `notifications` レコードを INSERT し、グループ全メンバーの `fcm_tokens` へ `type=begit_time` の FCM data メッセージを送信する shall。
2. The BeGit 通知バックエンドは1スプリントあたり1ユーザー1通知のみ許可し、`UNIQUE(sprint_id, sent_by)` 制約に違反する場合は 409 Conflict を返す shall。
3. If 同一スプリント内に `sent_at + 1h > now()` を満たすアクティブな BeGit Time! 通知が存在する, then the BeGit 通知バックエンドは新規発行を拒否し 409 Conflict（例:「別のチャレンジが進行中です」）を返す shall。
4. While アクティブなチャレンジが存在しない（直近通知の `sent_at + 1h <= now()`）, the BeGit 通知バックエンドは同一ユーザー以外の新規発行を許可する shall。
5. The BeGit 通知バックエンドは時間的非共存の判定をサービス層で行い、DB の `UNIQUE` 制約とは独立に評価する shall（時間条件は `UNIQUE` で表現できないため）。

---

### Requirement 2: ② Nice Work! 検知通知（GitHub Webhook 駆動）

**Objective:** As a チャレンジに応答した本人, I want GitHub での最初の行動を検知して「Nice Work!」を受け取れる, so that 写真投稿フローへ誘導され BeReal 型の投稿を完了できる

#### Acceptance Criteria

1. When BeGit 通知バックエンドが署名検証済みの `push` / `issues`(action=opened) / `pull_request_review` Webhook を受信した, the BeGit 通知バックエンドはリポジトリ名から対応グループを特定し、送信者が当該グループの `group_members` に属するかを判定する shall。
2. If Webhook 送信者が対応グループのメンバーでない, then the BeGit 通知バックエンドは ② を発火せず 200 OK で終了する shall。
3. When メンバーによる検知対象アクティビティを受理した, the BeGit 通知バックエンドは検知時刻以前で最新の同一スプリント内 BeGit Time! 通知を anchor として特定する shall。
4. If anchor となる BeGit Time! 通知が存在しない（チャレンジ未発行）, then the BeGit 通知バックエンドは ② を発火せず 200 OK で終了する shall。
5. When 当該チャレンジ（anchor）に対し当該ユーザーの投稿/下書きが未作成である, the BeGit 通知バックエンドは GitHub から取得した検知データ（`commit_count`/`additions`/`deletions`/`repo_full_name`/`branch_name`/`latest_commit_message`、`post_type` は `commit`/`issue`/`review`）を `posts` の draft 状態として作成し `notification_id` に anchor を紐づける shall。
6. While 当該チャレンジで当該ユーザーが既に投稿/下書きを保持している, the BeGit 通知バックエンドは追加のアクティビティを無視し ② を再発火しない shall（1チャレンジ1人1回、`posts.UNIQUE(notification_id,user_id)` で冪等）。
7. When 下書きを作成した, the BeGit 通知バックエンドは検知時刻と anchor の `sent_at + 1h` を比較し、検知が期限内なら `on_time`・期限超なら `late` を確定する shall。
8. When `on_time`/`late` を確定した, the BeGit 通知バックエンドは行動した本人のみへ `type=nice_work`（`notification_id`/`draft_post_id`/`status` を含む、ios-guide §2 準拠）の FCM data メッセージを送信する shall（グループ全体へは送らない）。

---

### Requirement 3: ③ チャレンジ終了通知（Cron、発行+1h）

**Objective:** As a グループメンバー, I want チャレンジ開始から1時間後に結果サマリを受け取れる, so that 誰が On Time だったかをチームで共有し盛り上がれる

#### Acceptance Criteria

1. When ある BeGit Time! 通知の `sent_at + 1h` に到達した, the BeGit 通知バックエンドは Cron 起点でそのチャレンジの結果サマリ（On Time/Late/Missed の集計）を算出する shall。
2. When 結果サマリを算出した, the BeGit 通知バックエンドはグループ全メンバーへ `type=challenge_end`（`group_id`/`notification_id` を含む、ios-guide §2 準拠）の FCM data メッセージを送信する shall。
3. The BeGit 通知バックエンドは同一チャレンジに対する ③ を1回のみ送信する shall（Cron の再実行で重複送信しない）。
4. The BeGit 通知バックエンドは ③ の結果サマリにおける各メンバーのステータスを、既存 `GET /groups/:id/notifications/:nid` と同一の判定基準（On Time/Late/Missed）で算出する shall。

---

### Requirement 4: ④⑤⑥ スプリント系通知（Cron 駆動）

**Objective:** As a グループメンバー, I want スプリントの節目（3日前/終了/開始）に通知を受け取れる, so that スプリントのリズムを意識して活動できる

#### Acceptance Criteria

1. When スプリントの `ends_at` の3日前に到達した, the BeGit 通知バックエンドはグループ全メンバーへ `type=sprint_reminder`（`group_id`/`sprint_id`）の FCM data メッセージを送信する shall。
2. When スプリントの `ends_at` に到達した, the BeGit 通知バックエンドはスプリント全体の結果まとめを算出しグループ全メンバーへ `type=sprint_end`（`group_id`/`sprint_id`）の FCM data メッセージを送信する shall。
3. When 既存 Cron が次スプリントを INSERT した, the BeGit 通知バックエンドはグループ全メンバーへ `type=sprint_start`（`group_id`/`sprint_id`=新スプリント）の FCM data メッセージを送信する shall。
4. The BeGit 通知バックエンドは ④⑤⑥ の各通知をスプリント1件につき各種別1回のみ送信する shall（Cron の再実行で重複送信しない）。

---

### Requirement 5: ⑦ リアクション/コメント通知（ソーシャル）

**Objective:** As a 投稿者, I want 自分の投稿への反応を通知で受け取れる, so that チームのフィードバックに気づける

#### Acceptance Criteria

1. When 他ユーザーが投稿にリアクションを付けた, the BeGit 通知バックエンドは投稿者本人へ `type=reaction`（`group_id`/`post_id`/`actor_login`）の FCM data メッセージを送信する shall。
2. When 他ユーザーが投稿にコメントを付けた, the BeGit 通知バックエンドは投稿者本人へ `type=comment`（`group_id`/`post_id`/`actor_login`）の FCM data メッセージを送信する shall。
3. If リアクション/コメントの実行者が投稿者本人である, then the BeGit 通知バックエンドは通知を送信しない shall（自己通知の抑制）。

---

### Requirement 6: FCM ペイロード契約（data メッセージ・type 拡張）

**Objective:** As a iOS クライアント, I want すべての通知で `type` と所定フィールドが入った data メッセージを受け取れる, so that `AppDelegate` で `type` を見て確実に画面分岐できる

#### Acceptance Criteria

1. The BeGit 通知バックエンドは全通知の FCM ペイロードに共通フィールド `type`（`begit_time`/`nice_work`/`challenge_end`/`sprint_reminder`/`sprint_end`/`sprint_start`/`reaction`/`comment` のいずれか）と `group_id` を含める shall。
2. The BeGit 通知バックエンドは FCM data メッセージの全フィールドを文字列キー/文字列値で送信する shall（数値も文字列化、FCM data message 制約準拠）。
3. The BeGit 通知バックエンドは `type` 別の追加フィールドを ios-guide.md §2 の表（例: `nice_work` は `notification_id`/`draft_post_id`/`status`）に一致させる shall。
4. If FCM トークンが存在しない/送信に失敗した, then the BeGit 通知バックエンドは当該受信者をスキップしても発火元の処理（API レスポンス/Cron 実行）を失敗させない shall（ベストエフォート送信）。

---

### Requirement 7: 下書き（draft）状態の管理とフィード除外

**Objective:** As a システム, I want 投稿の draft 状態を写真の有無と独立に管理できる, so that 確定前の検知データがフィードに漏れず、写真なし memo 投稿が誤って下書き扱いにならない

#### Acceptance Criteria

1. The BeGit 通知バックエンドは `posts` に draft を表す明示的な状態（`status='draft'` または `is_draft`）を保持し、写真（`photos`）の有無で draft を判定しない shall。
2. When ② の検知で draft を作成した, the BeGit 通知バックエンドは当該レコードを draft 状態で永続化する shall。
3. While 投稿が draft 状態である, the BeGit 通知バックエンドはフィード API（`GET /groups/:id/posts`）の結果から当該投稿を除外する shall。
4. When ユーザーが下書きを確定した（投稿確定 API）, the BeGit 通知バックエンドは draft 状態を解除しフィードに表示可能にする shall。
5. The BeGit 通知バックエンドは draft 状態の解除/確定に必要な API（下書き取得・確定）を OpenAPI 契約として提供する shall（正確なスキーマは OpenAPI が正）。

---

### Requirement 8: Webhook イベント拡張と冪等性

**Objective:** As a システム, I want Webhook を安全かつ冪等に受信し issues も検知できる, so that 二重発火なく ② を駆動できる

#### Acceptance Criteria

1. The BeGit 通知バックエンドはグループ作成時の Webhook 登録イベントに `issues` を追加する shall（現状 `push` / `pull_request_review` に加える）。
2. When `POST /webhook/github` を受信した, the BeGit 通知バックエンドは `X-Hub-Signature-256` を HMAC-SHA256 で検証し、一致しない場合は 403 Forbidden を返す shall。
3. When 署名検証に成功した, the BeGit 通知バックエンドは `X-GitHub-Delivery` を `github_webhook_deliveries` に INSERT し、重複（再配信）の場合は処理をスキップして 200 OK を返す shall。
4. The BeGit 通知バックエンドは同一アクティビティに対する ② の発火を、`github_webhook_deliveries` の delivery_id 記録と `posts.UNIQUE(notification_id,user_id)` の二重防御で1回に限定する shall。
5. If 対応グループが見つからない, then the BeGit 通知バックエンドは 200 OK を返してイベントを無視する shall。

---

### Requirement 9: Cron 基盤と結果サマリ算出

**Objective:** As a システム, I want 時刻起点の通知（③④⑤⑥）を駆動する Cron 基盤を持てる, so that ユーザー操作や Webhook に依らない時刻ベース通知を確実に発火できる

#### Acceptance Criteria

1. The BeGit 通知バックエンドは Cron トリガーから起動し、発火すべき通知（③: 発行+1h 到達、④: ends_at−3日 到達、⑤: ends_at 到達、⑥: 次スプリント生成）を判定する処理経路を持つ shall。
2. When Cron が実行された, the BeGit 通知バックエンドはその時点で発火条件を満たし未送信の通知のみを送信する shall。
3. The BeGit 通知バックエンドは結果サマリ（③/⑤）に On Time/Late/Missed の集計を含める shall。
4. The BeGit 通知バックエンドは Cron 実行が重複・再試行されても各通知を高々1回だけ送信できるよう、送信済みを判別する状態を持つ shall。

---

## 設計フェーズへの申し送り事項（design.md §9 残論点）

本要件で「前提を確定したもの」と「設計フェーズで詰めるもの」を区別して記録する。要件本文の WHAT は確定済み、ここでは HOW に踏み込む論点を扱う。

### S-1. ③/⑤ 結果サマリの算出責務 — **設計フェーズ送り**
- 要件側の確定: サマリは On Time/Late/Missed の集計を含み（Req3.3/Req9.3）、③ の各メンバー判定は `GET /groups/:id/notifications/:nid` と同一基準とする（Req3.4）。
- 設計で決める: 既存 `GetNotificationStatus`（[notification_service.go](../../../backend/internal/service/notification_service.go)）のロジックを共通関数として再利用するか、Cron 側で独立集計するか。⑤（スプリント全体まとめ）の集計対象（全通知横断/投稿数等）の定義。

### S-2. 冪等性の最終確認 — **前提確定（実装検証は設計/実装フェーズ）**
- 要件側の確定: ② は delivery_id 記録 + `posts.UNIQUE(notification_id,user_id)` の二重防御で1回に限定（Req8.4）。Cron 系は「送信済み判別状態」を持つ（Req9.4）。
- 設計で決める: 送信済みを記録する具体的手段（既存 `notifications`/`posts` への列追加か、新規 `notification_deliveries` テーブルか）。同一ユーザーに複数アクティビティが同時到達した際の競合（同一 anchor への並行 draft 作成）の防止方法。

### S-3. Missed と Cron の順序 — **設計フェーズ送り**
- 論点: ③/⑤ の集計時点と、既存設計の「Cron による Missed 確定（`posts` へ `status='missed'` を INSERT）」の順序関係。集計が Missed 確定の前か後かで「Missed 件数」がぶれる。
- 設計で決める: ③ 発火（発行+1h）時点で未投稿者を Missed として確定してから集計するか、集計は算出のみ行い Missed の永続化は別タイミングにするか。べき等性（S-2）と整合する順序を定義。

### S-4. 下書き（draft）の寿命 — **暫定前提を提示しつつ設計フェーズで最終決定**
- 暫定前提（要件としては Req7 で「draft はフィード除外」のみ確定）: draft が確定されないままスプリントが終了した場合、当該 draft は **フィード非表示のまま保持**し、③/⑤ 集計上は「未確定＝投稿なし」として Missed 相当に扱う方向を推奨。
- 設計で決める: 自動破棄するか保持するか、保持する場合の集計上の扱い（Missed 化の是非）、確定 API のべき等性。

### S-5. ④ の Cron 粒度 — **暫定前提を提示しつつ設計フェーズで最終決定**
- 暫定前提: ④（3日前リマインダー）は **日次バッチ**粒度を想定（「ends_at−3日」を日単位で判定）。③（発行+1h）はより細かい粒度（例: 分次）が必要。
- 設計で決める: Cron スケジュールの分割（時刻精度の異なる通知を単一 Cron で回すか複数に分けるか）、③ の +1h 判定に許容するレイテンシ、二重起動の抑止（Req9.4）。

---

<!-- Requirements は EARS 形式。残論点は上記「設計フェーズへの申し送り事項」に集約済み。 -->
