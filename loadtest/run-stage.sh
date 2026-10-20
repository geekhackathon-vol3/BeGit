#!/usr/bin/env bash
# 1段階ぶんの計測を、回の間を5分以上あけて繰り返す（#126 実施ルール）。
#
#   BASE_URL=https://... ./loadtest/run-stage.sh <USERS> [RUNS=3] [ROUNDS=1]
#
# 結果は loadtest/results/u<USERS>-run<N>.json（k6 のサマリー）と .log（標準出力）に保存する。
set -euo pipefail

USERS=${1:?usage: run-stage.sh <USERS> [RUNS] [ROUNDS]}
RUNS=${2:-3}
ROUNDS=${3:-1}
WAIT_SECONDS=${WAIT_SECONDS:-300}
: "${BASE_URL:?BASE_URL is required}"

cd "$(dirname "$0")"
mkdir -p results

# ウォームアップ（コールドスタートを計測に含めない）。回ごとに k6 の直前で実行する。
# 回の間の待機（5分）の間に、Go が D1 への接続を閉じ（IdleConnTimeout 90秒）、
# 段階の間が10分を超えるとコンテナも止まる（sleepAfter）ため。
warmup() {
  # 1. コンテナが起きるまで /healthz を叩く
  local ready=0
  for _ in $(seq 1 20); do
    if curl -fsS -m 30 "$BASE_URL/healthz" >/dev/null; then ready=1; break; fi
    sleep 10
  done
  if [ "$ready" -ne 1 ]; then
    echo "!!! ${BASE_URL}/healthz が応答しません" >&2
    exit 1
  fi
  # 2. D1 までの経路（api.cloudflare.com への接続）を温めるため、認証つきの /me を数回叩く
  #    （user001 は prepare.js で作成済みの前提）
  for _ in 1 2 3; do
    curl -fsS -m 30 -o /dev/null -H "Authorization: Bearer dev_user001" "$BASE_URL/me" || true
  done
}

for run in $(seq 1 "$RUNS"); do
  name="u${USERS}-run${run}"
  warmup
  echo "=== $(date '+%H:%M:%S') ${name} (USERS=${USERS} ROUNDS=${ROUNDS})"
  set +e
  k6 run \
    -e BASE_URL="$BASE_URL" -e USERS="$USERS" -e ROUNDS="$ROUNDS" \
    --summary-export "results/${name}.json" \
    scenario.js 2>&1 | tee "results/${name}.log"
  status=${PIPESTATUS[0]}
  set -e
  # 0 = 合格ラインを満たした、99 = 満たさなかった（計測としては成立しているので続行する）
  # それ以外（setup の失敗など）は計測が成立していないので止める
  if [ "$status" -ne 0 ] && [ "$status" -ne 99 ]; then
    echo "!!! k6 が終了コード ${status} で失敗しました。results/${name}.log を確認してください" >&2
    exit "$status"
  fi

  if [ "$run" -lt "$RUNS" ]; then
    echo "--- 上限の5分枠を空けるため ${WAIT_SECONDS}s 待機"
    sleep "$WAIT_SECONDS"
  fi
done
