#!/usr/bin/env bash
# 全段階（3 → 15 → 30 → 60 → 102人）をまとめて実行する（#126 実施ルール）。
#
#   BASE_URL=https://... caffeinate -dims ./loadtest/run-all.sh
#
# - 段階の間も5分（WAIT_SECONDS）あける
# - 初めて D1 の 429 が出た段階は3回実行して確定させ、それより上の段階は1回だけ実行する
# - 少人数の段階は周回数を増やしてサンプルを確保する（3人は5周、15人は2周）
set -euo pipefail

WAIT_SECONDS=${WAIT_SECONDS:-300}
: "${BASE_URL:?BASE_URL is required}"
export BASE_URL WAIT_SECONDS

cd "$(dirname "$0")"

# 段階ごとの「人数 周回数」
STAGES=("3 5" "15 2" "30 1" "60 1" "102 1")

# その段階の結果に D1 の 429 が含まれていたか（レスポンスのヘッダー由来と、サーバー全体の総数の差分の両方を見る）
stage_hit_rate_limit() {
  local users=$1 runs=$2 run total
  for run in $(seq 1 "$runs"); do
    total=$(jq '[.metrics.d1_rate_limited.count // 0, .metrics.d1_total_rate_limited.count // 0] | max' \
      "results/u${users}-run${run}.json")
    if [ "$total" -gt 0 ]; then return 0; fi
  done
  return 1
}

rate_limited=0
first=1
for stage in "${STAGES[@]}"; do
  read -r users rounds <<<"$stage"
  runs=3
  [ "$rate_limited" -eq 1 ] && runs=1

  if [ "$first" -eq 0 ]; then
    echo "### 段階の間も上限の5分枠を空けるため ${WAIT_SECONDS}s 待機"
    sleep "$WAIT_SECONDS"
  fi
  first=0

  echo "########## $(date '+%H:%M:%S') 段階 ${users}人（${runs}回 × ${rounds}周）"
  ./run-stage.sh "$users" "$runs" "$rounds"

  if [ "$rate_limited" -eq 0 ] && stage_hit_rate_limit "$users" "$runs"; then
    rate_limited=1
    echo "### ${users}人で初めて D1 の 429 が出たため、これより上の段階は1回だけ実行する"
  fi
done

echo "########## $(date '+%H:%M:%S') 全段階が終わりました。結果は loadtest/results/ にあります"
