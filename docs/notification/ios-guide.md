# BeGit; 通知 — iOS 実装ガイド（契約）

**バージョン:** 0.1.0 (Draft)
**作成日:** 2026-06-02
**読者:** フロント（iOS）担当
**関連:** [design.md（マスター設計）](design.md) / OpenAPI: [`ios/BeGit/BeGit/openapi.yaml`](../../ios/BeGit/BeGit/openapi.yaml)

---

## このドキュメントの役割

通知まわりで **iOS が実装する分** と、**バックエンドが保証する契約**（FCM ペイロード・API）だけをまとめたもの。バックエンドの内部設計（anchor 判定・冪等性・サマリ算出など）は読まなくてよい。詳細な全体像が知りたい時だけ [design.md](design.md) を参照。

- **API のリクエスト/レスポンスの正確な形は OpenAPI（[`openapi.yaml`](../../ios/BeGit/BeGit/openapi.yaml)）が正**。本ドキュメントは「どの通知で何を呼ぶか」の対応づけに集中する。
- **FCM ペイロードの `type` とフィールドが本ドキュメントの中心**。これがバックエンドとの契約。

---

## 1. 通知一覧（iOS 視点）

iOS が受け取る通知は7種類。受け取ったら `type` で分岐し、所定の画面へ遷移する。

| # | 通知 | `type` | 受信するか | タップ後にやること |
|---|---|---|---|---|
| ① | BeGit Time! | `begit_time` | 自分がメンバーのグループで誰かが発行 | チャレンジ中 → 投稿作成 UI（後述§3） |
| ② | Nice Work! | `nice_work` | **自分が**コミット/issue/review した時 | 写真撮影 → 下書きプレフィル → 投稿確定（§4） |
| ③ | チャレンジ終了 | `challenge_end` | グループ全員 | チャレンジ結果画面へ |
| ④ | スプリント終了3日前 | `sprint_reminder` | グループ全員 | スプリント概要画面へ |
| ⑤ | スプリント終了 | `sprint_end` | グループ全員 | スプリント結果画面へ |
| ⑥ | 新スプリント開始 | `sprint_start` | グループ全員 | スプリント概要画面へ |
| ⑦ | リアクション/コメント | `reaction` / `comment` | **自分の投稿に**他人が反応 | 該当投稿の詳細へ |

> 自分自身の操作では ⑦ は飛んでこない（バックエンドが自己通知を抑制）。

### 進捗メモ投稿（`memo`）とコメント機能（`comment`）は別物

| 用語 | 意味 | どこで出る |
|---|---|---|
| **post_type = `memo`**（進捗メモ投稿） | 「今は作業できないが近況を共有」する**投稿**。フィードに並ぶ | ① BeGit Time タップ → 投稿作成 UI（§3） |
| **コメント機能** | 他人の**投稿に付ける返信**（`comments`） | ⑦ `type: "comment"` 通知（投稿詳細へ遷移） |

- ① で作るのは **投稿（post_type=memo）**。⑦ の通知は **コメント機能**（誰かが自分の投稿にコメント）。別物。
- ※ かつて進捗メモ投稿の post_type は `comment` だったが、コメント機能との名前衝突を避けるため `memo` にリネーム済み。

---

## 2. FCM ペイロード契約

`AppDelegate.didReceiveRemoteNotification` で `data.type` を見て分岐する。`data` フィールドは文字列キー/値（FCM data message の制約）。

### 共通フィールド

| キー | 型 | 説明 |
|---|---|---|
| `type` | string | 上表の `type`。分岐の起点 |
| `group_id` | string | 対象グループ ID |

### `type` 別の追加フィールド

```jsonc
// ① begit_time（チャレンジ開始）
{ "type": "begit_time", "group_id": "12", "notification_id": "345", "sprint_id": "7" }

// ② nice_work（自分の初アクティビティ検知）
{
  "type": "nice_work", "group_id": "12",
  "notification_id": "345",   // anchor となった BeGit Time 通知
  "draft_post_id": "890",     // プレフィル元の下書き post（§4）
  "status": "on_time"         // "on_time" | "late"
}

// ③ challenge_end（発行+1h、結果サマリ）
{ "type": "challenge_end", "group_id": "12", "notification_id": "345" }

// ④⑤⑥ sprint 系
{ "type": "sprint_reminder", "group_id": "12", "sprint_id": "7" }  // 終了3日前
{ "type": "sprint_end",      "group_id": "12", "sprint_id": "7" }  // 終了
{ "type": "sprint_start",    "group_id": "12", "sprint_id": "8" }  // 新スプリント開始

// ⑦ reaction / comment（自分の投稿への反応）
{ "type": "reaction", "group_id": "12", "post_id": "890", "actor_login": "octocat" }
{ "type": "comment",  "group_id": "12", "post_id": "890", "actor_login": "octocat" }
```

> 数値も FCM data では文字列で届く点に注意（パース時に Int 変換）。確定したフィールド名・追加項目はバックエンド実装時に本表を更新する。

---

## 3. ① BeGit Time! タップ時の導線（iOS 実装）

- チャレンジ中（発行から1h以内）に通知をタップしたら、**投稿作成 UI** を開く。
- コミット等ができない人向けに、**memo（進捗メッセージ）タイプの投稿** をこの UI から作成できるようにする。
  - 「今は作業できないけど近況だけ共有」というユースケース。
  - post_type は `memo`。写真は任意（無くても投稿成立）。

---

## 4. ② Nice Work! タップ時の導線（iOS 実装の中心）

バックエンドが GitHub アクティビティ（commit/issue/review）を検知すると、**下書き（draft）post** をサーバー側に作成し、`draft_post_id` を載せて Nice Work! を Push する。

iOS のフロー：

1. `nice_work` 通知をタップ → アプリ起動。
2. `draft_post_id` の下書きを取得（commit 数・変更行数・repo・branch・最新コミットメッセージ等がプレフィル済み）。
3. **写真撮影**画面へ（BeReal 型）。
4. 撮影した写真 + プレフィル内容で **投稿を確定**（draft 解除）。
5. `status`（`on_time` / `late`）に応じてバッジ表示（On Time / Late）。

> 下書きの取得・確定 API の正確な形は OpenAPI を参照。draft は「写真の有無」ではなく明示的な状態で管理されているので、写真なし＝下書きという前提で UI を組まないこと。

---

## 4.5 BeGit Time! 進行中バナー / 途中中断 / アプリ内からのカメラ導線

Push をタップしなくても「今チャレンジ中かどうか」「自分は何をすべきか」が分かるように、`GET /groups/:id` が進行中チャレンジを返す。iOS はリポジトリ画面（Dashboard）を開くたびにこれを見てバナーを出す。

### 4.5.1 `GET /groups/:id` の `active_challenge`

進行中（発行から1h以内 かつ 未中断）の BeGit Time! があるときだけ `active_challenge` が付く。無ければキー自体が無い（`null` ではなく省略）。`GET /groups`（一覧）には付かない。

```json
{
  "id": 8, "name": "...", "members": [ ... ],
  "active_challenge": {
    "notification_id": 8,
    "sprint_id": 5,
    "sent_by": { "user_id": 23, "login": "riochin", "avatar_url": "https://..." },
    "sent_at": "2026-09-18T01:25:46Z",
    "ends_at": "2026-09-18T02:25:46Z",
    "can_end": true,
    "my_post": { "post_id": 41, "is_draft": true, "status": "on_time" }
  }
}
```

| フィールド | 意味 | iOS での使い方 |
|---|---|---|
| `sent_by` | 発行者。グループを離脱済みなら `login` は空文字 | バナーに「riochin が BeGit Time! 発行中」 |
| `sent_at` / `ends_at` | 発行時刻 / 締め切り（発行 + 1h）。RFC3339 UTC | `ends_at - now` で残り時間カウントダウン。0 になったらグループを再取得 |
| `can_end` | 要求ユーザーが発行者か | `true` のときだけ「終了する」ボタンを出す |
| `my_post` | 自分のこの通知への投稿。無ければ省略 | 下表 |

`my_post` による導線：

| `my_post` | 状態 | 表示 / 遷移 |
|---|---|---|
| 省略 | まだ何もしていない | 「投稿する」→ 投稿作成（memo 可、§3 と同じ） |
| `is_draft: true` | ② Nice Work! の下書きがある（GitHub 活動を検知済み） | 「撮影して投稿」→ 既存の deep-link ルート `.notificationNiceWorkDraft(groupId:, draftPostId: my_post.post_id, status: my_post.status)` と同じ画面（Push タップと同じフロー、§4） |
| `is_draft: false` | 投稿済み | 「投稿済み（On Time / Late）」バッジ。`status` は `on_time` / `late` |

進行中バナーが出ている間、`MakeNotificationView` の送信ボタンは無効化してよい（送ってから 409 を待つ必要がない）。

### 4.5.2 途中中断 `POST /groups/:id/notifications/:nid/end`

発行者だけが、進行中のチャレンジを途中で終了できる。「締め切りを今にする」扱いで、

- 終了時点で未投稿のメンバーは Missed、それ以降の投稿は Late になる（`GET …/notifications/:nid` のステータスに即反映）。
- ③ challenge_end（結果サマリ）の Push が 1 分以内にグループ全員へ届く（毎分 Cron が拾う）。
- 終了後は **直ちに次の BeGit Time! を発行できる**（同じスプリントでも 409 にならない）。

| レスポンス | 意味 | iOS の扱い |
|---|---|---|
| `200` `{ id, sprint_id, sent_at, ended_at }` | 終了した | グループを再取得 → バナーが消える |
| `403` | 発行者以外 | ボタンを出さない（`can_end` で制御）。出てしまった場合は再取得 |
| `404` | 通知がこのグループのものでない | 再取得 |
| `409` | もう進行中でない（中断済み / 1h 経過済み） | 再取得（バナーは消えるはず） |

`ended_at` は `POST /groups/:id/notifications` の 201 レスポンスにも（中断済みの場合のみ）付く同じ `NotificationJSON` のフィールド。

### 4.5.3 iOS 実装メモ

- 生成クライアントの操作名: `getGroupsId`（`active_challenge` は `Components.Schemas.Handler_ActiveChallengeJSON`）、`postGroupsIdNotificationsNidEnd`。`openapi-generator-config.yaml` の `filter.paths` には追加済み。
- バナーの残り時間はクライアント時計で計算してよいが、`ends_at` 到達・アプリ復帰時・終了ボタン押下後は必ず `GET /groups/:id` を再取得して状態を合わせる。
- `active_challenge` の取得はサーバー側でベストエフォート（失敗してもグループ詳細は 200 で返る）。キーが無い = 進行中なし、として扱えばよい。

---

## 5. iOS が叩く API（対応づけ）

正確なスキーマは [`openapi.yaml`](../../ios/BeGit/BeGit/openapi.yaml) が正。通知との対応のみ示す。

| タイミング | 呼ぶ API（概略） | 備考 |
|---|---|---|
| アプリ起動 / トークン更新 | `PUT /me/fcm-token` | FCM トークン登録（既存 Req6） |
| BeGit Time! を発行する | `POST /groups/:id/notifications` | 進行中チャレンジがあると **409**（時間非共存ルール）→ UI でハンドリング |
| リポジトリ画面を開く | `GET /groups/:id` | `active_challenge` があれば進行中バナー（§4.5.1）。`my_post.is_draft` でカメラ導線 |
| チャレンジを途中で終了する（発行者） | `POST /groups/:id/notifications/:nid/end` | `can_end: true` のときだけ。終了後は即再発行可（§4.5.2） |
| チャレンジ結果を見る | `GET /groups/:id/notifications/:nid` | On Time/Late/Missed の各メンバー状況 |
| Nice Work! 後に投稿確定 | 下書き取得 → 投稿確定（OpenAPI 参照） | `draft_post_id` を使用 |
| memo 投稿 | 投稿作成 API（post_type=`memo`） | §3 |

---

## 6. 実装チェックリスト（iOS）

- [x] `AppDelegate` で `type` 別の画面遷移を実装（§1・§2 の7種）※遷移先は #55 ではスタブ、中身は #56/#57
- [x] FCM data ペイロードのパース（文字列→Int 変換含む）
- [ ] ① memo タイプ投稿 UI
- [ ] ② Nice Work! → 下書き取得 → 撮影 → 確定フロー
- [ ] On Time / Late バッジ表示
- [ ] `POST /groups/:id/notifications` の 409（進行中チャレンジ）ハンドリング
- [x] `GET /groups/:id` の `active_challenge` → 進行中バナー（残り時間・発行者）と送信ボタン無効化（§4.5.1）※ `ActiveChallengeBannerView` / `RepositoryDashboardViewModel` / `MakeNotificationViewModel`
- [x] `can_end: true` のとき「終了する」→ `POST …/notifications/:nid/end` → グループ再取得（§4.5.2）
- [x] `my_post.is_draft: true` のとき、アプリ内から `.notificationNiceWorkDraft` へ遷移する「撮影して投稿」導線（§4.5.1）
- [ ] ⑦ リアクション/コメント通知 → 投稿詳細遷移

---

_FCM ペイロードのフィールドはバックエンド実装の進行に合わせて確定・更新する。変更時は本ドキュメントと [design.md](design.md) を同期すること。_
