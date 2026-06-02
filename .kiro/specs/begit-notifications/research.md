# Research & Design Decisions — begit-notifications

## Summary
- **Feature**: `begit-notifications`（バックエンドスコープ）
- **Discovery Scope**: Extension（既存 begit-backend-api への統合）
- **Key Findings**:
  - Cloudflare Workers の Cron Trigger は `scheduled()` ハンドラを起動でき、`getContainer(env.BEGIT_API, "begit-api-singleton")` 経由で既存 Workers Container（Go サーバー）に到達できる。既存の `fetch` 転送パターン（`X-Internal-*` 秘密ヘッダー注入）をそのまま再利用し、内部専用 HTTP ルート `POST /internal/cron` を叩く方式が最小差分。
  - 既存 `fcm.Client` は `notification{title,body}` のみ送信し `data` を送れない。ios-guide §2 契約（`type` 等の data フィールド）充足のため `data map[string]string` 送信の追加が全7通知の前提。
  - `GetNotificationStatus`（On Time/Late/Missed 算出）は ③ チャレンジ結果サマリへ共通化再利用できる。送信済み判別の状態は既存に無く、新規 `notification_deliveries` テーブルで一元化するのが S-2/S-3/S-4 を一括で解く。

## Research Log

### Cloudflare Workers Cron → Workers Container 起動経路
- **Context**: ③④⑤⑥ は時刻起点。現状 `wrangler.toml` に `[triggers]` なし、`src/index.ts` に `scheduled()` なし、Go に Cron 受け口なし（gap-analysis Research #1）。
- **Sources Consulted**:
  - [Scheduled Handler](https://developers.cloudflare.com/workers/runtime-apis/handlers/scheduled/)
  - [Cron Triggers](https://developers.cloudflare.com/workers/configuration/cron-triggers/)
  - [Cron Container example](https://developers.cloudflare.com/containers/examples/cron/)
  - [Multiple Cron Triggers](https://developers.cloudflare.com/workers/examples/multiple-cron-triggers/)
- **Findings**:
  - `scheduled(controller, env, ctx)` が cron 起点で実行される。`controller.cron` で「どの cron 式が発火したか」を判別でき、複数スケジュールを単一 Worker で振り分け可能。
  - Cron Container 例では `getContainer(env.X)` でインスタンス取得 → `container.start()` / `container.fetch()`。本プロジェクトの Container は常時 HTTP サーバー（`sleepAfter=10m`）なので `getContainer(...).fetch(new Request(...))` で内部ルートを叩くのが自然。
  - `wrangler dev --test-scheduled` が `/__scheduled` ルートを露出しローカルテスト可能。
- **Implications**:
  - `src/index.ts` に `scheduled()` を追加し、`fetch` と同じ `X-Internal-*` ヘッダー群＋`X-Cron-Secret` を付与して `POST /internal/cron?kind=<minutely|daily>` を Container へ送る。
  - Go 側は `/internal/cron` を **Bearer 認証なし・`X-Cron-Secret` 一致時のみ受理**（公開ルートにしない）。`controller.cron` の振り分けは `kind` クエリで Go に伝える。

### FCM HTTP v1 data メッセージ
- **Context**: Req6。`type`/`group_id` 等を iOS が `data` から読む（ios-guide §2）。
- **Findings**: FCM HTTP v1 の `message` は `notification`（表示用 title/body）と `data`（文字列 KV）を併存可能。data 値は文字列のみ（数値は文字列化）。既存 `fcmMessageRequest` 構造体に `Data map[string]string` を追加し `omitempty` で送る。
- **Implications**: `fcm.Notification` を拡張するか、`SendToTokens` に `data` 引数を追加。後者だと既存呼び出し（① など）も data 付与に移行できる。

## Architecture Pattern Evaluation

| Option | Description | Strengths | Risks / Limitations | Notes |
|--------|-------------|-----------|---------------------|-------|
| A 全面拡張 | 既存 service を直接拡張 | 新規最少 | webhook/notification service 肥大、Cron 受け皿なし | 単独では閉じない |
| B 新規分離 | 通知ドメインを新パッケージ化 | 関心分離・テスト容易 | 既存 SendNotification と二重管理 | 配線増 |
| **C Hybrid（採用）** | 既存拡張＋Cron経路/payloadビルダ/draftを新規 | バランス・新ライフサイクル分離 | 計画調整コスト | gap-analysis 推奨と一致 |

## Design Decisions

### Decision: Cron 経路は内部 HTTP ルート `/internal/cron`（kind 振り分け）
- **Alternatives Considered**:
  1. `container.start(envVars)` で都度起動しバッチ実行 — 常駐 HTTP サーバーと二重管理になる。
  2. `getContainer(...).fetch("/internal/cron")` で常駐サーバーに委譲 — 既存配線を最大流用。
- **Selected Approach**: (2)。`scheduled()` が `X-Internal-*`＋`X-Cron-Secret` 付きで `POST /internal/cron?kind=minutely|daily` を送る。Go 側 `cron_service` が kind に応じ ③（minutely: 発行+1h 到達判定）と ④⑤⑥（daily: ends_at 近傍・スプリント遷移）を処理。
- **Rationale**: 既存 `index.ts` の secret 転送・Container 配線をそのまま使え、Go 側もハンドラ追加で済む。
- **Trade-offs**: 内部ルートの保護を `X-Cron-Secret` に依存（Worker のみが知る値）。Bearer 体系と別系統。
- **Follow-up**: `X-Cron-Secret` は Workers Secret として登録。`/internal/cron` を bearer ミドルウェアから除外しつつ外部到達を遮断する（Worker 経由のみ）。

### Decision: 送信済み状態を新規 `notification_deliveries` テーブルで一元管理（S-2/S-3/S-4）
- **Alternatives Considered**:
  1. `notifications`/`sprints` に列追加（`challenge_end_sent_at` 等）— 種別ごとに列が増え拡張性が低い。
  2. 新規 `notification_deliveries(kind, ref_id, sent_at, UNIQUE(kind, ref_id))` — 全 Cron 通知の冪等を一元化。
- **Selected Approach**: (2)。`kind ∈ {challenge_end, sprint_reminder, sprint_end, sprint_start}`、`ref_id` は通知ID(③) or スプリントID(④⑤⑥)。INSERT の UNIQUE 違反＝送信済みでスキップ（Webhook の delivery_id と同じ冪等パターン）。
- **Rationale**: Req3.3/4.4/9.4 の「高々1回」を DB 制約で保証。Cron 再実行・二重起動に強い。
- **Trade-offs**: テーブル1つ追加。
- **Follow-up**: ③ は anchor 通知ID、④⑤⑥ は sprint_id を ref に。

### Decision: Missed/Cron 順序（S-3）= ③ は「算出のみ」、Missed 永続化はスプリント終了時
- **Context**: ③（発行+1h）時点で未投稿者を `posts.status='missed'` 永続化すると、その後の Late 投稿（検知窓はスプリント終了まで）と矛盾する。
- **Selected Approach**: ③ チャレンジ結果サマリは **算出のみ**（その時点の On Time/Late/Missed を集計して送信、DB に missed を書かない）。`posts.status='missed'` の永続化は ⑤（スプリント終了）時にそのスプリントの未確定/未投稿を確定。
- **Rationale**: design.md §4.1「Late でも鳴る／検知窓はスプリント終了まで」と整合。③ 時点の Missed は速報、確定は終了時。
- **Trade-offs**: ③ の Missed は暫定値（後で Late に変わり得る）。サマリ文言で「現時点」を含意。
- **Follow-up**: ⑤ まとめ集計の対象＝スプリント内全 anchor 横断 + 確定 missed。

### Decision: draft 寿命（S-4）= フィード非表示のまま保持、未確定は集計上 Missed 相当
- **Selected Approach**: 未確定 draft は破棄せず保持（`posts.is_draft=1`）。フィードは常に除外。スプリント終了時の集計では「未確定＝投稿なし」として Missed 相当に数える（確定済みのみ On Time/Late）。
- **Rationale**: ユーザーが後から確定できる余地を残しつつ、フィード健全性と集計の一貫性を確保。
- **Trade-offs**: 孤児 draft が DB に残る（将来のクリーンアップ余地）。
- **Follow-up**: 確定 API はべき等（既確定の再確定は no-op で 200）。

### Decision: Cron 粒度（S-5）= 2系統（minutely / daily）
- **Selected Approach**: ③（発行+1h、分精度が要る）は `* * * * *`（毎分）相当の minutely cron。④⑤⑥（ends_at−3日/ends_at/新スプリント、日精度で十分）は daily cron。`controller.cron` で振り分け。
- **Rationale**: ③ の +1h を分単位の遅延で発火でき、④ は日次で十分・コスト最小。
- **Trade-offs**: cron 定義が2つ。最小間隔・課金は Cloudflare の制約に従う。
- **Follow-up**: 毎分 cron の実行コスト・冪等（notification_deliveries）で二重送信を完全抑止。

## Risks & Mitigations
- 毎分 Cron のコスト/負荷 — 対象は「発行+1h を跨いだ未送信 anchor」のみを軽量クエリで抽出、`notification_deliveries` で冪等。
- ② の同一 anchor 並行 draft 競合（S-2/Research #6）— `posts.UNIQUE(notification_id,user_id)` を INSERT で先取りし、違反時は ② を再発火しない（既存 ErrConstraintViolation パターン）。
- `/internal/cron` の外部到達 — Worker のみが知る `X-Cron-Secret` で受理判定し、公開ルーティングに載せない。
- FCM data 移行で既存 ① 通知の互換性 — `notification` は残しつつ `data` を追加（併存）。iOS は data を読む。

## References
- [Scheduled Handler · Cloudflare Workers](https://developers.cloudflare.com/workers/runtime-apis/handlers/scheduled/) — `scheduled()` と `controller.cron`
- [Cron Triggers · Cloudflare Workers](https://developers.cloudflare.com/workers/configuration/cron-triggers/) — `[triggers] crons` 設定
- [Cron Container example · Cloudflare Containers](https://developers.cloudflare.com/containers/examples/cron/) — `getContainer` からの Container 起動
- [Multiple Cron Triggers · Cloudflare Workers](https://developers.cloudflare.com/workers/examples/multiple-cron-triggers/) — 複数スケジュールの振り分け
- `docs/notification/design.md` v0.3.0 / `docs/notification/ios-guide.md` v0.1.0 — 契約と全体設計
