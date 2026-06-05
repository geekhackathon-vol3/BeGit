# BeGit; 通知文面カタログ（title / body）

**バージョン:** 0.1.0
**作成日:** 2026-06-02
**読者:** バックエンド / フロント（iOS）両方
**関連:** [design.md（マスター設計）](design.md) / [ios-guide.md（iOS 契約）](ios-guide.md) / [api-notification-map.md（API↔叩く瞬間）](api-notification-map.md)
**真実のソース:** [`backend/internal/service/notification_payload.go`](../../backend/internal/service/notification_payload.go)（`Build*` 関数）

---

## このドキュメントの役割

各通知の **表示文面（FCM `notification.title` / `notification.body`）** を1か所で確認できるようにする。文面は `notification_payload.go` の `Build*` 関数が唯一の生成元で、本書はその一覧と「**どこが固定文でどこが動的か**」を明示する。

- `data`（`type` / `group_id` など、iOS の分岐に使う機械可読フィールド）は [ios-guide.md §2](ios-guide.md) / [api-notification-map.md](api-notification-map.md) が担当。本書は **人間が読む title/body** に集中する。
- 文面を変えたい場合は `notification_payload.go` の該当 `Build*` を編集し、`notification_payload_test.go` を更新する。

---

## 1. 文面一覧（実装済み）

| # | `type` | 受信者 | Title | Body | 動的部分 |
|---|---|---|---|---|---|
| ① | `begit_time` | グループ全員 | `🐙 BeGit Time!` | `今、なに作ってる？チームへの通知が届きました` | なし（固定） |
| ② | `nice_work` | 行動した本人 | `🎉 Nice Work!` | `いい仕事！写真を撮ってチームへシェアしよう` | なし（固定）※ |
| ③ | `challenge_end` | グループ全員 | `🏁 チャレンジ終了` | `結果が出ました。みんなの様子を見てみよう` | なし（固定）※ |
| ④ | `sprint_reminder` | グループ全員 | `⏳ スプリント終了3日前` | `ラストスパート！残り3日です` | なし（固定） |
| ⑤ | `sprint_end` | グループ全員 | `🏆 スプリント終了` | `今回の結果をチェックしよう` | なし（固定）※ |
| ⑥ | `sprint_start` | グループ全員 | `🚀 新スプリント開始` | `新しいスプリントが始まりました` | なし（固定） |
| ⑦a | `reaction` | 投稿者本人 | `❤️ リアクションが届きました` | `{actor_login} があなたの投稿に反応しました` | **`{actor_login}`** を差し込み |
| ⑦b | `comment` | 投稿者本人 | `💬 コメントが届きました` | `{actor_login} があなたの投稿にコメントしました` | **`{actor_login}`** を差し込み |

> **Title の絵文字**: 各通知タイプの性格に合わせて先頭に付与（①🐙 ②🎉 ③🏁 ④⏳ ⑤🏆 ⑥🚀 ⑦❤️/💬）。①は GitHub の Octocat に寄せた 🐙。変更は `notification_payload.go` の各 `Build*` の `Title` を編集する。

> **※ 注意（現状の制約）**: ② の `on_time`/`late`、③⑤ の結果サマリ（On Time/Late/Missed の人数）は **`data` 側にしか無く、title/body には反映されていない**（本文は固定文）。詳細は §3。

---

## 2. 動的に変わるのは ⑦ のみ

現状、表示本文に値を差し込んでいるのは ⑦（リアクション/コメント）の `actor_login` だけ。

```
reaction → "octocat があなたの投稿に反応しました"
comment  → "octocat があなたの投稿にコメントしました"
```

他の①〜⑥はすべて**完全な固定文**。受信者名・グループ名・スプリント番号などは本文に含めていない（iOS が `data` を使って画面側で表現する想定）。

---

## 3. 設計意図と「本文に入っていない情報」

design.md では以下を想定していたが、**現状の Push 本文は固定文**で、数値・状態は `data` 経由（または未表示）になっている。意図的な切り分けと将来拡張候補を明記する。

| 項目 | design.md の想定 | 現状の実装 | 補足 |
|---|---|---|---|
| ② On Time / Late バッジ | 本人に On Time/Late を伝える（design §4） | `data.status`（`on_time`/`late`）で渡す。**本文は固定** | バッジ表示は iOS 側（ios-guide §4）。本文に出さないのは設計どおり |
| ③ チャレンジ結果サマリ | 「今回は ○人が On Time！」等（design §5） | 集計はするが**本文は固定文**。人数はログ出力のみ | サマリ数値を body に載せるなら `BuildChallengeEnd` を集計引数付きに拡張する余地 |
| ⑤ スプリント結果まとめ | スプリント全体の結果まとめ（design §6） | **本文は固定文** | 同上。まとめ内容を body に載せるなら拡張 |

> つまり「**通知をタップして画面で結果を見る**」前提の文面設計になっている（プッシュ本文は誘導役）。③ の達成人数などをプッシュ本文自体に出したい場合は別途要件化が必要。

---

## 4. 文面を変更/追加するには

1. [`backend/internal/service/notification_payload.go`](../../backend/internal/service/notification_payload.go) の対象 `Build*`（例: `BuildChallengeEnd`）の `Title`/`Body` を編集。
2. `data` フィールドを増減する場合は [ios-guide.md §2](ios-guide.md) と [api-notification-map.md](api-notification-map.md) を同期更新（フロントとの契約）。
3. テスト `backend/internal/service/notification_payload_test.go` を更新（data フィールドの一致・全値文字列を担保）。
4. サマリ数値を本文に入れるなど**引数を増やす**場合は、呼び出し元（`cron_service` / `nicework_service` 等）の `Build*` 呼び出しも合わせて変更する。

---

## 5. まとめ（1行で）

- 文面は `notification_payload.go` の `Build*` が唯一の生成元。①〜⑥は固定文、**動的差し込みは ⑦ の `actor_login` のみ**。
- ②の status・③⑤の結果サマリは `data`／ログにあり、**Push 本文には出していない**（タップ後の画面で見る設計）。本文に出したい場合は拡張が必要。
