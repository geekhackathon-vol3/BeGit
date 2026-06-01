#!/usr/bin/env bash
#
# seed-dev.sh — dev API（DEV_MODE=true の begit-dev Worker）にシードデータを投入する。
# 公開 API を順に叩いて user / group / notification / post を作成する。
# スタブ GitHub クライアントのおかげで実 GitHub なしで全て成功する。スモークテスト兼用。
#
# 使い方:
#   DEV_URL=https://begit-dev.118029-ichikama.workers.dev ./scripts/seed-dev.sh
#   （make seed-dev からも呼ばれる）
#
# 再実行しても安全（group/notification/post の重複は警告のみで続行）。

set -uo pipefail

DEV_URL="${DEV_URL:-https://begit-dev.118029-ichikama.workers.dev}"
REPO="begit-dev/playground"

echo "🌱 seeding dev API: $DEV_URL"

# JSON の特定キーを抽出するヘルパー（python3 を使用）
jget() { python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)" 2>/dev/null; }

# --- 1. dev ログイン（alice / bob のトークン取得）-----------------------------
ALICE_TOKEN=$(curl -sf -X POST "$DEV_URL/auth/dev" -H 'Content-Type: application/json' \
  -d '{"login":"alice"}' | jget "['token']")
BOB_TOKEN=$(curl -sf -X POST "$DEV_URL/auth/dev" -H 'Content-Type: application/json' \
  -d '{"login":"bob"}' | jget "['token']")

if [ -z "${ALICE_TOKEN:-}" ] || [ -z "${BOB_TOKEN:-}" ]; then
  echo "❌ /auth/dev からトークンを取得できませんでした。DEV_URL とデプロイ状態を確認してください。"
  exit 1
fi
echo "  ✓ tokens: alice=$ALICE_TOKEN bob=$BOB_TOKEN"

# --- 2. alice がグループ作成（bob はスタブ collaborators で自動参加）----------
GROUP_ID=$(curl -sf -X POST "$DEV_URL/groups" \
  -H "Authorization: Bearer $ALICE_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"repo_full_name\":\"$REPO\",\"name\":\"Dev Playground\"}" | jget "['id']")

if [ -z "${GROUP_ID:-}" ]; then
  # 既に存在（409）の可能性 → 一覧から repo 一致のグループ id を取得
  echo "  ℹ グループ作成をスキップ（既存の可能性）。一覧から取得します。"
  GROUP_ID=$(curl -sf "$DEV_URL/groups" -H "Authorization: Bearer $ALICE_TOKEN" \
    | python3 -c "import sys,json; gs=json.load(sys.stdin); print(next((g['id'] for g in gs if g.get('repo_full_name')=='$REPO'), ''))" 2>/dev/null)
fi

if [ -z "${GROUP_ID:-}" ]; then
  echo "❌ グループ id を取得できませんでした。"
  exit 1
fi
echo "  ✓ group id=$GROUP_ID"

# --- 3. alice が通知を発行（1スプリント1人1回。再実行時の 409 は許容）---------
NOTIF_BODY=$(curl -s -X POST "$DEV_URL/groups/$GROUP_ID/notifications" \
  -H "Authorization: Bearer $ALICE_TOKEN")
NOTIF_ID=$(echo "$NOTIF_BODY" | jget "['id']")
if [ -z "${NOTIF_ID:-}" ]; then
  echo "  ℹ 通知は作成済みのようです（再実行時は正常）。notification_id なしで投稿します。"
  NOTIF_FIELD="null"
else
  echo "  ✓ notification id=$NOTIF_ID"
  NOTIF_FIELD="$NOTIF_ID"
fi

# --- 4. alice / bob が投稿作成（UNIQUE 制約による 409 は許容）------------------
for who in alice bob; do
  case "$who" in
    alice) TOK="$ALICE_TOKEN" ;;
    bob)   TOK="$BOB_TOKEN" ;;
  esac
  CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$DEV_URL/groups/$GROUP_ID/posts" \
    -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' \
    -d "{\"body\":\"$who の dev 投稿\",\"notification_id\":$NOTIF_FIELD,\"github_login\":\"$who\",\"repo_full_name\":\"$REPO\"}")
  echo "  ✓ post by $who → HTTP $CODE"
done

echo ""
echo "✅ seed 完了"
echo "   フィード確認: curl $DEV_URL/groups/$GROUP_ID/posts -H \"Authorization: Bearer $ALICE_TOKEN\""
