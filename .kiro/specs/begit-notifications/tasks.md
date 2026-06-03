# 実装計画 — begit-notifications

> 全タスクは TDD（テスト先行）で進める。依存は番号順が基本。`(P)` は直前の同階層タスクと並行実行可能（境界が非重複）。
> 対象はバックエンドのみ（Go / Workers）。iOS は Out of Scope（契約は ios-guide.md）。

## 1. 基盤: スキーマと外部連携の土台

- [x] 1.1 D1 マイグレーション 0003（draft 列 + notification_deliveries）
  - `posts` に `is_draft INTEGER NOT NULL DEFAULT 0` を追加（既存行はデフォルト 0 で後方互換）
  - `notification_deliveries(id, kind, ref_id, sent_at, UNIQUE(kind, ref_id))` を新規作成
  - `wrangler d1 migrations apply`（dev）でマイグレーションが適用でき、両変更がスキーマに反映されることを確認
  - _Requirements: 7.1, 3.3, 4.4, 9.4_

- [x] 1.2 (P) FCM クライアントの data メッセージ対応
  - 送信リクエストに `data`（文字列キー/文字列値の map）を併存させ、`notification`（title/body）と同時に送れるようにする
  - data が空の場合は従来どおり notification のみ送信する（既存 ① 通知の後方互換）
  - テスト: data 付き送信で FCM リクエストボディに data が含まれることを検証
  - _Requirements: 6.1, 6.2, 6.4_
  - _範囲: pkg/fcm_

- [x] 1.3 (P) Webhook 登録イベントに issues を追加
  - グループ作成時の GitHub Webhook 登録イベントへ `issues` を加える（既存 push / pull_request_review に追加）
  - テスト: 登録ペイロードの events に push / pull_request_review / issues の3種が含まれることを検証
  - _Requirements: 8.1_
  - _範囲: pkg/github_

## 2. コア: リポジトリ層（クエリと冪等状態）

- [x] 2.1 (P) 送信済み冪等リポジトリ（notification_deliveries）
  - `(kind, ref_id)` の INSERT を行い、UNIQUE 違反は「送信済み」として識別できる返り値にする
  - テスト: 同一 (kind, ref_id) の2回目 INSERT が送信済みと判定されることを検証
  - _Requirements: 3.3, 4.4, 9.4_
  - _範囲: notification_delivery_repository_
  - _依存: 1.1_

- [x] 2.2 (P) 通知リポジトリに anchor / アクティブ通知クエリ追加
  - 「同一スプリント内・指定時刻以前で最新の BeGit Time! 通知」を取得するクエリ（② anchor 用）
  - 「同一スプリント内に `sent_at + 1h > now()` の通知が存在するか」の判定クエリ（① 非共存用）
  - テスト: 複数通知から時刻以前最新が選ばれること、アクティブ判定の境界（ちょうど +1h）を検証
  - _Requirements: 1.3, 2.3_
  - _範囲: notification_repository_

- [x] 2.3 (P) 投稿リポジトリの draft 対応（除外・作成・確定・取得）
  - フィード一覧クエリで `is_draft = 0` のみ返す（draft を除外）
  - draft 投稿の作成（`is_draft=1`、`notification_id` 紐付け）、確定（`is_draft=0` へ更新、べき等）、取得
  - テスト: draft がフィードに出ないこと、確定後に出ること、再確定が no-op であることを検証
  - _Requirements: 7.1, 7.2, 7.3, 7.4_
  - _範囲: post_repository_
  - _依存: 1.1_

- [x] 2.4 (P) スプリントリポジトリに Cron 走査クエリ追加
  - 「ends_at の3日前に到達したスプリント」「ends_at に到達したスプリント」「新規生成されたスプリント」を抽出するクエリ
  - テスト: 各境界（3日前ちょうど・終了ちょうど）で対象が抽出されることを検証
  - _Requirements: 4.1, 4.2, 4.3_
  - _範囲: sprint_repository_

## 3. コア: サービス層（発火ロジック）

- [x] 3.1 通知ペイロードビルダ（type 別 data 構築）
  - ①〜⑦ の各 `type` に対し ios-guide §2 準拠の data（共通 `type`/`group_id` ＋ type 別フィールド）を構築する。数値は文字列化する
  - 各 type に対応する表示用 notification（title/body）も組み立てる
  - テスト: 各 type の data フィールドが ios-guide §2 と一致し、全値が文字列であることを検証
  - _Requirements: 6.1, 6.2, 6.3_
  - _範囲: notification_payload_
  - _依存: 1.2_

- [x] 3.2 ① BeGit Time! 発行への時間的非共存ルール追加
  - 発行時にアクティブな（`sent_at + 1h > now()`）チャレンジが存在すれば 409 Conflict を返す（サービス層判定）
  - `UNIQUE(sprint_id, sent_by)` 違反も従来どおり 409。発行成功時は `type=begit_time` の data 付き FCM をグループ全員へ送る
  - テスト: アクティブ通知ありで 409、無しで発行成功＋begit_time data 送信、同一ユーザー再発行の扱いを検証
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_
  - _範囲: notification_service_
  - _依存: 2.2, 3.1_

- [x] 3.3 ② Nice Work! 発火サービス
  - メンバー判定 → anchor 特定 → 初アクティビティ冪等（draft INSERT を UNIQUE で先取り、違反は skip）→ on_time/late 確定 → 本人のみへ `nice_work` data 送信
  - anchor 無し / 非メンバー / 既発火 はいずれも no-op（送信しない）
  - テスト: 本人のみ送信（グループ非送信）、冪等 skip、on_time/late 境界、anchor 無し no-op を検証
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8_
  - _範囲: nicework_service_
  - _依存: 2.2, 2.3, 3.1_

- [x] 3.4 ③④⑤⑥ Cron サービス
  - kind=minutely: `sent_at+1h` 到達かつ未送信の anchor を抽出 → サマリ算出（On Time/Late/Missed、既存ステータス算出を共通化再利用）→ delivery INSERT 成功時のみ全員へ `challenge_end`
  - kind=daily: ④ 3日前→`sprint_reminder`、⑤ 終了→missed 確定＋まとめ→`sprint_end`、⑥ 新スプリント→`sprint_start`。各々 `notification_deliveries` で冪等
  - ③ 時点では missed を永続化しない（確定は ⑤）。FCM 失敗はベストエフォート（Cron 実行は失敗にしない）
  - テスト: minutely で challenge_end 1回（再実行 skip）、daily 各種別の冪等、③ が missed を書かないこと、⑤ で missed 確定を検証
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 4.1, 4.2, 4.3, 4.4, 9.2, 9.3, 9.4_
  - _範囲: cron_service_
  - _依存: 2.1, 2.4, 3.1_

- [x] 3.5 (P) ⑦ リアクション/コメント通知（自己抑制）
  - リアクション/コメント成功後に投稿者を特定し、実行者が投稿者本人でなければ `reaction`/`comment` data を投稿者へ送る
  - 自己操作では送信しない。FCM 失敗は本処理（リアクション/コメント登録）を失敗させない
  - テスト: 他者操作で投稿者へ送信、自己操作で非送信、FCM 失敗時も登録成功を検証
  - _Requirements: 5.1, 5.2, 5.3, 6.4_
  - _範囲: reaction_service, comment_service_
  - _依存: 3.1_

## 4. 統合: 配線とエンドポイント

- [x] 4.1 Webhook サービスへ issues 受理と ② 委譲を統合
  - `issues`(action=opened) を処理対象に追加し、push/issues/pull_request_review でグループ・メンバー判定後に Nice Work! 発火サービスへ委譲する
  - delivery_id 冪等・署名検証・対応グループ無し時 200 の既存挙動を維持
  - テスト: 3イベントで ② が駆動されること、非メンバー/グループ無しで 200 skip、重複 delivery で skip を検証
  - _Requirements: 2.1, 2.2, 8.2, 8.3, 8.4, 8.5_
  - _範囲: webhook_service_
  - _依存: 3.3, 1.3_

- [x] 4.2 下書き取得・確定エンドポイントと feed 除外の公開
  - 下書き取得 API と確定 API（draft 解除）を追加し、フィード一覧が draft を除外することをエンドポイント経由で保証する
  - OpenAPI（openapi.yaml）に両エンドポイントと is_draft 挙動を反映する
  - テスト: 確定前は feed 非表示・取得可能、確定後は feed 表示、確定のべき等を API レベルで検証
  - _Requirements: 7.3, 7.4, 7.5_
  - _範囲: post_service, handler_
  - _依存: 2.3_

- [x] 4.3 Cron エンドポイントとサービス配線
  - `POST /internal/cron?kind=` を `X-Cron-Secret` 一致時のみ受理（bearer 不要、公開ルーティングに載せない、定数時間比較）し cron_service を呼ぶ
  - 新 service/repository（payload/nicework/cron/delivery）と social/notification への FCM 依存注入を buildHandler に配線。Config に CronSecret を追加し X-Cron-Secret ヘッダーから補完
  - テスト: secret 一致で 200・kind 振り分け、secret 不一致で 403、kind 不正で 400 を検証
  - _Requirements: 9.1, 9.2_
  - _範囲: cron_handler, container.go, main.go_
  - _依存: 3.4_

- [x] 4.4 Workers scheduled ハンドラと Cron トリガー設定
  - `src/index.ts` に `scheduled()` を追加し、`X-Internal-*` ＋ `X-Cron-Secret` 付きで `POST /internal/cron?kind=minutely|daily` を Container へ転送（`controller.cron` で minutely/daily 振り分け）
  - `wrangler.toml` に `[triggers] crons`（毎分 + 日次）と X-Cron-Secret（dev は var、本番は secret 運用メモ）を追加
  - 観察可能: `wrangler dev --test-scheduled` の `/__scheduled` 経由で cron 経路が Container まで到達する
  - _Requirements: 9.1_
  - _範囲: src/index.ts, wrangler.toml_
  - _依存: 4.3_

## 5. 検証: 結合・E2E

- [x] 5.1 通知フロー結合テスト
  - Webhook（push/issues/pr_review）→ ② 発火（本人のみ・冪等）、① 非共存（409）、⑦（自己抑制）の結合を検証
  - draft 確定 → feed 表示遷移の結合を検証
  - 観察可能: 上記結合テストがグリーンで通る
  - _Requirements: 1.1, 1.3, 2.1, 2.6, 2.8, 5.1, 5.3, 7.3, 7.4_
  - _依存: 4.1, 4.2_

- [x] 5.2 Cron 冪等 E2E
  - `--test-scheduled` 経由で minutely → challenge_end が1回のみ送信（再実行で skip）、daily → sprint_reminder/end/start が各1回、③ が missed を永続化せず ⑤ で確定することを検証
  - 観察可能: Cron 二重起動シナリオでも notification_deliveries により重複送信ゼロ
  - _Requirements: 3.1, 3.2, 3.3, 4.1, 4.2, 4.3, 4.4, 9.2, 9.3, 9.4_
  - _依存: 4.4_
