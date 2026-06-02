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
- ① BeGit Time タップ → comment タイプ投稿作成 UI。
- ② Nice Work タップ → 写真撮影 → 下書きプレフィル → 投稿確定の画面フロー。
- On Time / Late バッジ表示。
- バックエンドは上記 UI が依存する **FCM ペイロード契約** と **API（OpenAPI）** を提供する責務のみを負う。

### 関連既存 spec
- [`begit-backend-api`](../begit-backend-api)（Req3 通知発行 / Req5 Webhook 受信 / Req6 FCM トークン管理）を参照・一部更新する。

## Requirements
<!-- Will be generated in /kiro-spec-requirements phase -->
