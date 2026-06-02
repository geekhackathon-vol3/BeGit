# Gap Analysis — begit-notifications

**対象**: `begit-notifications`（バックエンドスコープのみ）
**前提**: 既存 `begit-backend-api` 実装（Go / Cloudflare Workers Container + D1 + R2 + FCM）に対する差分分析。

## 分析サマリー

- 既存資産は流用度が高い（通知発行・Webhook受信・投稿・ステータス算出の骨格は実装済み）が、**通知の data ペイロード送信**・**Cron 基盤**・**Nice Work! 発火ロジック**・**draft 状態**・**ソーシャル通知**の5領域がまるごと不足。
- 最大の構造的ギャップは **Cron 基盤**（③④⑤⑥）。`wrangler.toml` に `[triggers]` なし、`src/index.ts` に `scheduled()` ハンドラなし、Go 側に Cron 受け口なし。Workers→Container の scheduled 呼び出し方法は **Research Needed**。
- 次点は **FCM data メッセージ**。現 `fcm.Client` は `notification{title,body}` のみで `data` を送れず、ios-guide §2 契約（`type` 等の data フィールド）を満たせない。全7通知に影響する横断的ギャップ。
- 推奨アプローチは **Hybrid（Option C）**: 既存 service/repository を拡張しつつ、Cron 経路・draft・通知ペイロード組み立てを新規コンポーネントとして切り出す。

---

## 1. 現状調査（Current State）

### アーキテクチャ
- Cloudflare Workers (`src/index.ts`, TS) がエントリ。Secrets/vars を `X-Internal-*` ヘッダーに載せて Workers Container 内の Go HTTP サーバー（gin）へ `fetch` 転送（[index.ts:34-52](../../../backend/src/index.ts)）。
- Go 側は `cmd/server/container.go` の `buildHandler()` で pkg→repository→service→handler→routing を配線（[container.go:24](../../../backend/cmd/server/container.go)）。
- 層依存方向は handler→service→repository を厳守。命名 snake_case（Go）。テストは各層 `_test.go` 同居。

### 再利用可能な既存資産
| 資産 | 場所 | 流用可否 |
|---|---|---|
| BeGit Time! 発行 | `notification_service.SendNotification` | ◎ 拡張（非共存・data 化） |
| ステータス算出 On Time/Late/Missed | `notification_service.GetNotificationStatus` | ◎ ③ サマリで共通化再利用 |
| Webhook 受信・署名検証・冪等(delivery_id) | `webhook_handler` + `webhook_service` + `github_webhook_deliveries` | ○ ② 発火ロジック追加 |
| anchor 用 notif 取得 | `notification_repository`（Create/GetByID のみ） | △ 「スプリント内・時刻以前で最新」クエリ追加要 |
| 投稿/フィード | `post_service` / `post_repository`（ListByGroupID/HasPostedInSprint/GetByUserAndNotification） | ○ draft 列・除外・確定追加 |
| FCM 送信 | `fcm.Client.SendToTokens` | △ data 非対応、要拡張 |
| FCMトークン取得（グループ単位） | `fcm_token_repository.GetTokensByGroupID` | ◎ そのまま |
| リアクション/コメント | `reaction_service`/`comment_service` | ○ FCM 依存注入＋発火追加 |
| Webhook 登録イベント | `github.RegisterWebhook`（`events: [push, pull_request_review]`） | △ `issues` 追加要 |

---

## 2. 要件→資産マップ（ギャップ分類: Missing / Unknown / Constraint）

| 要件 | 必要な技術要素 | 既存資産 | ギャップ |
|---|---|---|---|
| Req1 時間非共存 | スプリント内アクティブ通知の有無判定 | SendNotification / notif_repo | **Missing**: `notif_repo` に「sprint内で sent_at+1h>now の通知」クエリ＋service判定 |
| Req2 Nice Work! | メンバー判定→anchor特定→初活動冪等→draft保存→on_time/late→本人FCM | webhook_service(sprint更新のみ) | **Missing**: 発火ロジック全体。anchorクエリ、draft作成、本人トークン取得 |
| Req2 issues検知 | `issues`(opened) 処理 | webhook_service が push/PRreview のみ通過 | **Missing**: イベント許可＋登録イベント追加 |
| Req3 チャレンジ終了 | Cron(発行+1h)→サマリ→全員FCM、1回限定 | GetNotificationStatus | **Missing**: Cron経路、送信済み判別、ペイロード |
| Req4 スプリント系 | Cron(ends_at-3d / ends_at / 新規INSERT)→全員FCM | sprint_repo(現行Cron言及あるが未実装) | **Missing**: Cron経路全般、スプリント走査クエリ、⑤まとめ算出 |
| Req5 ⑦ソーシャル | リアクション/コメント→投稿者本人FCM、自己抑制 | reaction/comment service | **Missing**: FCM依存注入、投稿者特定、自己抑制、actor_login |
| Req6 FCM契約 | data メッセージ（type/group_id等、全文字列） | fcm.Client(notificationのみ) | **Missing**: `data map[string]string` 送信、ペイロードビルダ |
| Req7 draft | posts に draft状態、feed除外、確定API | posts(draft列なし)/post_repo | **Missing**: マイグレーション、ListByGroupID除外、取得/確定エンドポイント |
| Req8 冪等 | delivery_id + posts.UNIQUE 二重防御 | 両方存在 | **Constraint**: 既存で概ね充足。同一anchor並行draft競合は要検証 |
| Req9 Cron基盤 | Workers scheduled→Container、送信済み状態 | なし | **Missing + Unknown**: scheduled→Container 呼出方法が Unknown |

---

## 3. 実装アプローチ（Options）

### Option A: 既存 service/handler を全面拡張
- notification_service / webhook_service / reaction_service / comment_service / fcm.Client を直接拡張。
- ✅ 新規ファイル最少、既存パターン踏襲。❌ webhook_service と notification_service が肥大化（②は判定が多段）。Cron はそもそも受け皿がなく拡張だけでは閉じない。

### Option B: 通知ドメインを新規パッケージに分離
- `internal/service/notifier`（ペイロード組み立て＋送信）と Cron 専用 service/handler を新設。
- ✅ 関心分離・テスト容易。❌ 既存 SendNotification 等との二重管理リスク、配線増。

### Option C: Hybrid（推奨）
- **拡張**: `fcm.Client` に data 対応追加 / `notification_service` に非共存判定 / `webhook_service` に ② 発火 / `reaction|comment_service` に FCM 注入。
- **新規**: ①〜⑦ の data ペイロードを組み立てる **通知ペイロードビルダ**（`pkg/fcm` か `internal/service/notification_payload.go`）、**Cron 経路**（`src/index.ts` `scheduled()` ＋ Go 側 `POST /internal/cron`（内部限定）＋ `cron_service`）、**draft マイグレーション＋確定エンドポイント**。
- ✅ 既存流用と新規分離のバランス。Cron という新ライフサイクルを独立させられる。❌ 計画の調整コスト。

**推奨**: Option C。

---

## 4. 実装複雑度・リスク

| 領域 | Effort | Risk | 根拠 |
|---|---|---|---|
| FCM data 対応 + ペイロードビルダ | S | Low | 既存 fcmMessageRequest に `data` 追加するのみ。契約は ios-guide §2 で確定 |
| ① 時間非共存 | S | Low | クエリ1本＋service分岐。既存パターン |
| ② Nice Work! 発火 | L | Medium | 多段ロジック（メンバー/anchor/冪等/draft/判定/送信）。並行発火の競合に注意 |
| issues イベント拡張 | S | Low | 許可リスト＋RegisterWebhook events 追加 |
| draft 状態 + feed除外 + 確定API | M | Medium | D1マイグレーション＋feedクエリ改修＋新規エンドポイント＋OpenAPI更新 |
| ⑦ ソーシャル通知 | M | Low | 依存注入＋投稿者特定＋自己抑制。既存反応/コメント経路に追加 |
| ③④⑤⑥ Cron 基盤 | L〜XL | **High** | Workers scheduled→Container 呼出が未知。送信済み状態設計、時刻精度（③+1h vs ④日次）が絡む |
| 送信済み(冪等)状態 | M | Medium | 列追加 or 新テーブル。S-2/S-3/S-4 と密結合 |

---

## 5. Research Needed（設計フェーズへ持ち越す調査項目）

1. **Workers Container の `scheduled()` 起動経路**: Cloudflare Workers の cron トリガー（`[triggers] crons`）から Durable Object/Container の Go HTTP サーバーへどう到達するか。`getContainer(...).fetch("/internal/cron")` で内部 POST する方式が有力だが、scheduled ハンドラ内での Container 起床・認証（`X-Internal-*` 転送）を要確認。**[Research Needed]**
2. **Cron 粒度の分割**（S-5）: ③(+1h, 分次精度) と ④(ends_at−3日, 日次) を単一 cron で回すか分割するか。Cloudflare cron の最小間隔と運用コスト。
3. **送信済み状態の保持手段**（S-2/S-3/S-4）: `notifications`/`sprints` への列追加か新規 `notification_deliveries` テーブルか。③/⑤/Missed 確定の順序（S-3）と整合する設計。
4. **⑤ スプリント全体まとめの集計定義**（S-1）: スプリント内の全チャレンジ横断集計 or 投稿総数等、何を「結果まとめ」とするか。
5. **draft 寿命の最終決定**（S-4）: 未確定 draft をスプリント終了時に破棄/保持/Missed化のいずれにするか。確定APIのべき等性。
6. **②の anchor 並行競合**: 同一ユーザーの複数 delivery が同時到達した際、同一 anchor への並行 draft 作成を `posts.UNIQUE(notification_id,user_id)` だけで防げるか（D1 のトランザクション特性）。

---

## 6. 設計フェーズへの推奨

- **採用アプローチ**: Option C（Hybrid）。
- **優先設計判断**:
  1. Cron 経路アーキテクチャ（Research #1）を最優先で確定 — ③④⑤⑥ すべてが依存。
  2. 送信済み状態のデータモデル（Research #3）を確定し、S-2/S-3/S-4 を一括で解く。
  3. FCM data ペイロードビルダの責務境界（`pkg/fcm` 拡張 vs `internal/service` 新規）を決める。
- **持ち越し調査**: 上記 Research #1〜#6 をそのまま設計フェーズの調査項目とする。requirements 末尾「設計フェーズへの申し送り事項」S-1〜S-5 と対応。
