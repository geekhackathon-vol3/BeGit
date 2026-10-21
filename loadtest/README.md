# 負荷試験（#126 Before 計測 / After 計測）

BeGit Time! の同時投稿を再現し、現行構成が合格ライン（全エンドポイントの p95 < 1秒、エラー率 < 1%）を満たすかを測る。
Before / After で同じスクリプトを使う。

## ファイル

| ファイル | 役割 |
|---|---|
| `prepare.js` | ユーザー（`user001`〜`user102`）とグループ（3人 × 34）を作る。`groups.json` と、メンバー登録用の `members.sql` を書き出す |
| `scenario.js` | 計測本体。トリガー → フィード取得 → 投稿作成 → 写真アップロード → 投稿確定 → リアクション |
| `lib.js` | 共通処理（API 呼び出し、`X-D1-*` ヘッダーの記録） |
| `run-stage.sh` | 1段階ぶんを、回の間を5分以上あけて繰り返す |
| `run-all.sh` | 全段階（3 → 102人）をまとめて実行する |
| `results/` | 計測結果（git 管理外） |

## 前提

- 計測先は `D1_METRICS=true` の環境。無効な環境でも動くが、D1 の内訳は取れない
  - 計測用の Wrangler 設定は `backend/wrangler.loadtest.toml.example` を `backend/wrangler.loadtest.toml` にコピーし、`<...>` を計測用アカウントの値に置き換えて作る（`.gitignore` 済み）
  - wrangler のコマンドには `-c wrangler.loadtest.toml` を付ける（例：`npx wrangler deploy -c wrangler.loadtest.toml`）
- `DEV_MODE=true`（`/auth/dev` を使う）と `BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT=true`
- GitHub / R2 / FCM はスタブ（認証情報を登録しない）
- k6 v2 以降

```sh
export BASE_URL=https://begit-loadtest.<subdomain>.workers.dev
cd loadtest
```

## 1. 準備（初回のみ）

3段階に分けて実行する。1と2は何度実行しても同じ状態になる。3は実行するたびに投稿が増えるので、流し直すのは途中で失敗したときだけにする。

```sh
# 1. ログイン（102人）と、各グループの1人目によるグループ作成（34件）。groups.json と members.sql を書き出す
k6 run -e BASE_URL=$BASE_URL prepare.js

# 2. 2・3人目を poster としてグループに登録する
(cd ../backend && CLOUDFLARE_ACCOUNT_ID=<計測用アカウントの ID> \
  npx wrangler d1 execute begit-loadtest-db -c wrangler.loadtest.toml --remote --yes --file ../loadtest/members.sql)

# 3. 1人1件のシード投稿（1周目からリアクション先があるように）
k6 run -e BASE_URL=$BASE_URL -e PHASE=seed prepare.js
```

- 2・3人目を API（`POST /groups`）で参加させないのは、スタブの GitHub が全員を同じユーザーとして返すため。全員が「GitHub 上の実オーナー」と判定され、owner の付け替えに入ってしまう
- 1と3は1.5秒間隔で直列に送る（合わせて238リクエスト、10分ほど）。D1 の 429 に当たったら65秒待って再試行する
- **終わったら5分以上待ってから計測を始める**（準備で使った上限の枠を空けるため）

## 2. 計測

```sh
./run-stage.sh 3 3 5      # 3人。少人数はサンプルが少ないので5周回す
./run-stage.sh 15 3 2
./run-stage.sh 30
./run-stage.sh 60
./run-stage.sh 102
```

引数は `<同時人数> [回数=3] [周回数=1]`。回の間は自動で5分（`WAIT_SECONDS`）待つ。

各回の直前に自動でウォームアップする（`/healthz` が応答するまで待ち、`user001` で `/me` を3回叩いて D1 までの接続を温める）。`user001` を使うため、先に `prepare.js` を実行しておくこと。

### 実施ルール（#126）

- 段階の間も5分以上あける（`run-stage.sh` は段階の中の回の間だけ待つ）
- **初めて D1 の 429 が出た段階は3回実行して確定させる。それより上の段階は回数1で実行し、全滅を確認したら打ち切る**
- 計測の直前にデプロイしない（デプロイも同じ枠を消費する）
- 計測中は OrbStack や重いアプリを止め、ネットワークを固定する。スリープは `caffeinate -dims` で止める

## 3. 結果の読み方

サマリー（`results/u<人数>-run<N>.json`）の主な指標。

| 指標 | 意味 |
|---|---|
| `http_req_duration{endpoint:*}` | エンドポイント別のレイテンシ（p50 / p95 / p99、件数） |
| `http_req_failed{endpoint:*}` | エンドポイント別のエラー率 |
| `d1_queries{endpoint:*}` | 1リクエストあたりの D1 クエリ数 |
| `d1_time_ms{endpoint:*}` | 1リクエストあたりの D1 の所要時間の合計 |
| `d1_rate_limited` | レスポンスに含まれていた D1 の 429 の回数。**Go 側で 500 に変換されるため、429 の判定はこちらで行う** |
| `d1_total_calls` / `d1_total_rate_limited` | 計測の前後で取った、サーバー全体の D1 呼び出し数（レスポンス後の非同期の呼び出しも含む） |

エンドポイント名：`trigger`、`feed`、`create_post`、`upload_photo`、`confirm_post`、`reaction`

合格ラインを満たさないと k6 は終了コード 99 を返す（`run-stage.sh` はそのまま次へ進む）。それ以外の失敗（setup でトリガーが通らないなど）では `run-stage.sh` が止まる。

### 注意

- 計測を重ねるとフィードの投稿が増えていく（1回につき1人1件）。フィード取得の結果を比べるときは件数の違いに注意する
- `setup` の `trigger` は全グループ同時（`http.batch`）、`teardown` の BeGit Time! の停止は合格ラインの判定に含めない

## 4. 後片付け（After 計測が終わってから）

計測用の環境は担当者個人の Cloudflare アカウントにある。After まで使い終わったら消す。

```sh
cd backend
export CLOUDFLARE_ACCOUNT_ID=<計測用アカウントの ID>
npx wrangler delete -c wrangler.loadtest.toml
npx wrangler d1 delete begit-loadtest-db
npx wrangler queues delete begit-loadtest-external-notifications
npx wrangler queues delete begit-loadtest-external-notifications-dlq
```

- ダッシュボードの API Tokens で、D1 用のトークン（`begit-loadtest-d1`）を削除する
- ほかで使わなければ、Workers Paid プランを Free に戻す（Worker を消しても月額は止まらない）
