# BeGit; 通知設計（マスター）

**バージョン:** 0.3.0 (Draft)
**作成日:** 2026-06-02
**最終更新:** 2026-06-02
**ステータス:** 設計まとめ（全体像の共有用マスター）
**関連:** [`../spec.md`](../spec.md) §5.2 / [`begit-backend-api`](../../.kiro/specs/begit-backend-api) / [`begit-notifications` spec](../../.kiro/specs/begit-notifications)

---

## 読者別ドキュメント構成

通知に関するドキュメントは読者別に3層に分かれる。**本ファイルは全体像を共有するマスター**。

| ドキュメント | 読者 | 役割 |
|---|---|---|
| **design.md**（本ファイル） | バックエンド・フロント両方 | 全体像・通知カタログ・設計判断の理由 |
| [**ios-guide.md**](ios-guide.md) | フロント（iOS） | 契約のみ：FCM `type` 別ペイロード / 遷移マップ / iOS が叩く API / 下書きプレフィルのデータ形 |
| [**begit-notifications spec**](../../.kiro/specs/begit-notifications) | バックエンド | バックエンド実装の SDD（API・Webhook・Cron・D1・FCM 送信・サマリ算出） |

**役割分担**：バックエンド実装はオーナー（SDD で管理）、iOS 実装はフロント担当。両者は **FCM ペイロード契約** と **API 契約** で接続する。iOS の画面実装は SDD spec の Out of Scope（ios-guide 参照）。

---

## 目的

BeGit; の通知を体系的に整理し、`begit-notifications` spec（バックエンド）と iOS 実装の共通土台とする。FCM の `type` 設計・Cron・Webhook 駆動を1か所で見通せるようにする。

---

## 1. 通知カタログ（全体像）

通知は **起点の基盤** で3カテゴリに分かれる。

| # | 通知 | カテゴリ | 起点 | 受信者 | メッセージ要旨 | 状態 |
|---|---|---|---|---|---|---|
| ① | **BeGit Time!** 発行 | A. チャレンジ | ユーザー操作 (`POST /groups/:id/notifications`) | グループ全員 | 「今なに作ってる？」/ 1h チャレンジ開始 | ✅定義済 (Req3) |
| ② | **Nice Work!** | A. チャレンジ | GitHub Webhook 検知 | **行動した本人だけ** | 「Nice Work! (On Time/Late)」 | 🔧設計済 |
| ③ | **チャレンジ終了通知** | A. チャレンジ | Cron（発行+1h） | グループ全員 | チャレンジ結果サマリ | 🆕スコープ入り |
| ④ | **スプリント終了3日前** | B. スプリント | Cron（ends_at−3日） | グループ全員 | 「スプリント終了まであと3日」 | 🆕スコープ入り |
| ⑤ | **スプリント終了通知** | B. スプリント | Cron（ends_at到達） | グループ全員 | スプリント結果まとめ | 🆕スコープ入り |
| ⑥ | **新スプリント開始通知** | B. スプリント | Cron（新スプリント生成時） | グループ全員 | 「新しいスプリントが始まりました」 | 🆕スコープ入り |
| ⑦ | **リアクション/コメント通知** | C. ソーシャル | ユーザー操作 | 投稿者本人 | 「○○さんがリアクションしました」 | 🆕スコープ入り |

- **A（チャレンジ系）**: ① はユーザー操作、② は Webhook 駆動、③ は Cron 駆動。BeGit Time サイクルを構成する。
- **B（スプリント系）**: すべて Cron 駆動。既存 `sprints` テーブルとスプリント Cron をそのまま使える。
- **C（ソーシャル系）**: 投稿への反応をトリガーとする。

### 用語の区別：進捗メモ投稿（`memo`）とコメント機能（`comment`）

かつて「進捗メモ投稿」の post_type が `comment` で、コメント機能と名前衝突していたが、post_type を **`memo` にリネーム済み**で衝突は解消された。両者は別物。

| 用語 | 実体 | 意味 | 関係する通知 |
|---|---|---|---|
| **post_type = `memo`**（進捗メモ投稿） | `posts` テーブルの1行（=投稿そのもの） | 「今は作業できないが近況を共有」する単独投稿。フィードに並ぶ | ① BeGit Time タップから作成（§3.3） |
| **コメント機能** | `comments` テーブルの1行（=投稿への返信） | 他人の投稿に付けるコメント／スレッド | ⑦ `type: "comment"` 通知（§7） |

- ① の「memo タイプ投稿 UI」は **post_type = memo**（投稿を作る）。
- ⑦ の FCM `type: "comment"` は **コメント機能**（誰かが自分の投稿にコメントを付けた）。両者は無関係。

### FCM ペイロード `type` 設計（契約。詳細は [ios-guide.md](ios-guide.md)）

`AppDelegate.didReceiveRemoteNotification` が `type` を見て遷移を分岐する。

| 通知 | `type` | タップ後の遷移（iOS 実装） |
|---|---|---|
| ① | `begit_time` | チャレンジ中なら投稿作成 UI（memo タイプ可）へ |
| ② | `nice_work` | 写真撮影 → 下書きプレフィル投稿確定へ。`status`(on_time/late)・下書き参照を含む |
| ③ | `challenge_end` | チャレンジ結果画面へ |
| ④⑤⑥ | `sprint_reminder` / `sprint_end` / `sprint_start` | スプリント概要画面へ |
| ⑦ | `reaction` / `comment` | 該当投稿の詳細へ |

---

## 2. 全体フロー（A. チャレンジ系）

```
[誰かが BeGit Time! を発行]  ← ①
   │  ※ 同スプリントにアクティブな(発行+1h以内)チャレンジがある間は発行不可（§3.2）
   └→ FCM でグループ全員へ Push「今なに作ってる？」/ 1h チャレンジ開始
        │
        ├─(A) 受信者が GitHub で行動した（commit / issue / review）
        │     └→ Webhook 検知 → その人の "初アクティビティ" か判定（§4）
        │          ├→ 検知データを下書き保存（posts の draft 状態）
        │          ├→ anchor 通知の sent_at + 1h と比較し On Time / Late を判定
        │          └→ 本人へ「Nice Work! (On Time/Late)」を Push  ← ②
        │               └→ タップ → アプリ → 写真撮影 → 下書きプレフィル → 投稿確定
        │
        └─(B) コミットできない人
              └→ BeGit Time! 通知をタップ（チャレンジ中）
                   └→「memo タイプの投稿作成 UI」へ → 進捗共有だけ投稿
        │
   [発行 + 1h 到達] ← Cron
        └→ チャレンジ結果サマリをグループ全員へ Push  ← ③
```

---

## 3. ① BeGit Time!（既存仕様 + 追加ルール）

### 3.1 既存仕様（begit-backend-api Req3・前提）
- `POST /groups/:id/notifications` で発行。当日スプリントを取得/作成し `notifications` に INSERT
- `UNIQUE(sprint_id, sent_by)` で **1スプリント1人1回**を保証（違反は 409 Conflict）。← **この制約は維持**
- 発行成功 → FCM でグループ全員へ Push
- ステータス算出：`On Time`(1h以内) / `Late`(1h超) / `Missed`(投稿なし)

### 3.2 【追加ルール】チャレンジの時間的非共存（合意）
- **同一スプリント内で、アクティブな（発行から1h以内の）チャレンジが存在する間は、新たな BeGit Time! を発行できない。**
- 判定は**サービス層**で行う（時間条件は DB の `UNIQUE` で表現できないため）。`POST /groups/:id/notifications` 時に「同スプリントに `sent_at + 1h > now()` の通知が存在するか」を確認し、存在すれば **409 Conflict**（例:「別のチャレンジが進行中です」）を返す。
- 効果：**任意の瞬間にアクティブなチャレンジは最大1つ** → ② の anchor が一意に定まる（§4.4）。
- 「1人1回」は維持されるため、各メンバーは1スプリント中に（重ならない範囲で）順番に発行できる。製品の「いつ打つか」の戦略性も保たれる。

### 3.3 iOS 側の導線（→ ios-guide.md）
- BeGit Time! 通知をチャレンジ中にタップ → **memo（message）タイプの投稿作成 UI** へ遷移し、「進捗共有だけ」投稿できる経路。実装は iOS 側（契約は ios-guide）。

---

## 4. ② Nice Work!（チャレンジ応答の検知通知）

本質：**サーバーが GitHub アクティビティを検知し「チャレンジに応答できたね！」を本人に返す通知**。

### 4.1 トリガー（合意）

| アクティビティ | GitHub イベント | post_type |
|---|---|---|
| コミット push | `push` | `commit` |
| Issue 発行 | `issues` (action=opened) | `issue` |
| PR レビュー | `pull_request_review` | `review` |

- **検知窓**：BeGit Time! 発行後 〜 スプリント終了まで
- **Late でも鳴る**：1h を超えた初アクティビティでも「Nice Work! (Late)」として送る
- **1人1回**：そのチャレンジで最初の1アクティビティのみ。以降は無視（冪等）。既存 `posts.UNIQUE(notification_id, user_id)` がそのまま効く

### 4.2 受信者
- **行動した本人だけ**（グループ全体には流さない）

### 4.3 下書き保存（合意：明示フラグで管理）
- 検知時に GitHub から取得したデータ（commit 数・additions・deletions・repo_full_name・branch・latest_commit_message 等）を **`posts` の draft 状態**として保存。
- **「写真の有無」を下書き判定に使わない**。理由：
  1. memo タイプ投稿は写真なしが正常 → 誤って下書き扱いになる
  2. 写真は R2 への非同期アップロード（`feat/backend-photo-upload-r2`）。確定済みでも一瞬「写真なし」になり得る
  3. フィードの可視性が `photos` JOIN 状態に引きずられて壊れやすい
- → `posts` に明示的な状態（`status='draft'` または `is_draft`）を持たせ、写真の有無と独立させる。フィード API は draft を除外する。
- ユーザーが「Nice Work!」をタップ → アプリ → 写真撮影 → 下書きをプレフィル → 投稿を確定（draft 解除）。BeReal 型。

### 4.4 anchor の特定（合意：時間非共存により一意）
- §3.2 の非共存ルールにより、任意時点でアクティブなチャレンジは最大1つ。
- **anchor 通知 = アクティビティ検知時刻以前で最新の BeGit Time! 通知**（同スプリント内）。
- 例：10:00 発行（〜11:00 が On Time 窓）。11:30 に初アクティビティ → anchor は 10:00 の通知、`Late` 判定。
- Nice Work! 発火時、この anchor の `id` を `posts.notification_id` に紐づけ、`sent_at + 1h` と比較して `on_time` / `late` を確定する。

---

## 5. ③ チャレンジ終了通知（合意：全員に結果サマリ）

- 起点：Cron。BeGit Time! 発行から **1h 経過時**に発火。
- 受信者：**グループ全員**。
- 内容：そのチャレンジの結果サマリ（例：「今回は ○人が On Time！」、On Time/Late/Missed の集計）。
- 盛り上げ重視。締切間際の「あと○分」催促は今回スコープ外（将来拡張）。

---

## 6. ④⑤⑥ スプリント系通知（Cron 駆動）

すべて既存 `sprints` テーブル＋スプリント Cron を起点とする。受信者はグループ全員。

| # | 通知 | トリガー | 内容 |
|---|---|---|---|
| ④ | スプリント終了3日前 | `ends_at − 3日` | 「スプリント終了まであと3日」リマインダー |
| ⑤ | スプリント終了通知 | `ends_at` 到達 | スプリント全体の結果まとめ |
| ⑥ | 新スプリント開始通知 | 次スプリント INSERT 時（既存 Cron が次スプリントを作成） | 「新しいスプリントが始まりました」 |

---

## 7. ⑦ リアクション/コメント通知（ソーシャル系）

- 起点：他ユーザーが投稿にリアクション／コメントを付けた時（ユーザー操作）。
- 受信者：**投稿者本人**。
- 自分自身の操作では発火しない（自己通知の抑制）。

---

## 8. 既存仕様への差分（spec 化で必要な変更）

1. **Webhook 登録イベントに `issues` を追加**（現状 `push` / `pull_request_review` のみ — Req2.3 / Req5）
2. **Webhook ハンドラに Nice Work! 発火ロジック**を追加（現状「スプリント情報更新」止まり — Req5.4/5.5）
   - 送信者=グループメンバー判定 → anchor 通知特定 → 初アクティビティ判定（冪等） → 下書き保存 → On Time/Late 判定 → 本人へ FCM Push
3. **BeGit Time! の時間非共存ルール**をサービス層に追加（§3.2）
4. **`posts` に draft 状態**を追加し、フィード API で除外（§4.3）
5. **Cron に通知トリガーを追加**：③（発行+1h）④（ends_at−3日）⑤⑥（スプリント遷移）
6. **iOS：memo タイプ投稿 UI への導線**（フロント実装、契約は ios-guide）
7. **FCM ペイロード `type` の拡張**：`begit_time` / `nice_work` / `challenge_end` / `sprint_*` / `reaction` / `comment`（§1）

---

## 9. 設計フェーズで詰める残論点

- **③/⑤ の結果サマリの算出責務**：既存 `GET /groups/:id/notifications/:nid` のステータス算出ロジックを再利用するか、Cron 側で別途集計するか。
- **冪等性の最終確認**：同一ユーザーに複数アクティビティが届いた場合、`posts.UNIQUE(notification_id, user_id)` ＋ `github_webhook_deliveries` の delivery_id 記録で二重発火を防げるか実装レベルで検証。
- **Missed と Cron の順序**：③/⑤ の集計時点と、Cron による Missed 確定（既存設計）の順序関係。
- **下書きの寿命**：draft が確定されないままスプリントが終わった場合の扱い（自動破棄 or Missed 化）。
- **④ の Cron 粒度**：3日前判定の実行頻度（日次バッチ等）。
- ~~**post_type `comment` のリネーム要否**~~：**解消済み**。コメント機能との名前衝突（§1 用語の区別）を解消するため、進捗メモ投稿の post_type を `comment` → `memo` にリネーム済み（schema・begit-backend-api spec・本ドキュメント群を同期更新済み）。

---

## 10. 次アクション

- [x] 通知カタログの体系化
- [x] ② Nice Work! の anchor・下書き・トリガーを確定
- [x] 追加通知③〜⑦のスコープ確定
- [x] ドキュメントを読者別に3層化（design / ios-guide / SDD spec）
- [ ] §9 の残論点を設計フェーズで詰める
- [ ] `/kiro:spec-requirements begit-notifications` でバックエンド要件を EARS 形式へ
