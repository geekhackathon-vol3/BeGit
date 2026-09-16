#!/usr/bin/env bash
# swaggo(--v3.1) が出力する openapi.yaml には、Apple swift-openapi-generator が依存する
# OpenAPIKit でパースできない不正な OpenAPI 3.1 構造が混ざる。iOS 取り込み用にそれを補正する。
# バックエンドの Go アノテーションには手を入れない方針のため、配布(=同期)後の iOS 側コピーに対して適用する。
#
# 補正内容（いずれも swaggo 由来の既知バグ。スパイクで実測して必要と確認した最小セット）:
#   1. `type: file` → `type: string`
#      OpenAPI 2.0 の `file` 型は 3.1 に存在せず、ドキュメント全体のデコードが失敗する。
#      （写真アップロード定義は後段で正しい multipart schema に正規化する）
#   2. 写真アップロードの requestBody を正しい multipart schema に置換
#      swaggo v2.0.0-rc5 は formData file を application/x-www-form-urlencoded に出力し、
#      multipart/form-data 側を空 object にするため、Swift の multipart 型が生成されない。
#   3. 空 URL の `externalDocs` ブロック削除
#      `url: ""` は不正な URL とみなされ、ルート Document のパースが失敗する。
#
# 使い方: sanitize-openapi-for-ios.sh <path-to-openapi.yaml>
set -euo pipefail

target="${1:?usage: sanitize-openapi-for-ios.sh <openapi.yaml>}"
[ -f "$target" ] || { echo "error: file not found: $target" >&2; exit 1; }

perl -0777 -i -pe '
  # 1. type: file -> type: string（インデント維持）
  s/^(\s*)type:\s*file[ \t]*$/${1}type: string/mg;
  # 2. 写真アップロードを swift-openapi-generator が型生成できる schema に直す。
  s{(  /groups/\{id\}/posts/\{postId\}/photos:\n.*?      requestBody:\n).*?(?=      responses:\n)}
   {$1        required: true\n        content:\n          multipart/form-data:\n            schema:\n              type: object\n              required:\n                - main\n              properties:\n                main:\n                  type: string\n                  format: binary\n                front:\n                  type: string\n                  format: binary\n}s;
  # 3. ルート直下の externalDocs ブロック（externalDocs: と 2 スペース字下げの子行）を削除
  s/^externalDocs:\n(?:[ ]{2}\S.*\n)*//mg;
' "$target"

echo "✅ iOS 向けに openapi 仕様をサニタイズしました: $target"
