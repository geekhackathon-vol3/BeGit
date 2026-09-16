#!/usr/bin/env bash
# swaggo(--v3.1) が出力する openapi.yaml には、Apple swift-openapi-generator が依存する
# OpenAPIKit でパースできない不正な OpenAPI 3.1 構造が混ざる。iOS 取り込み用にそれを補正する。
# バックエンドの Go アノテーションには手を入れない方針のため、配布(=同期)後の iOS 側コピーに対して適用する。
#
# 補正内容（いずれも swaggo 由来の既知バグ。スパイクで実測して必要と確認した最小セット）:
#   1. `type: file` → `type: string`
#      OpenAPI 2.0 の `file` 型は 3.1 に存在せず、ドキュメント全体のデコードが失敗する。
#   2. 空 URL の `externalDocs` ブロック削除
#      `url: ""` は不正な URL とみなされ、ルート Document のパースが失敗する。
#   3. /groups/{id}/posts/{postId}/photos の requestBody を multipart/form-data（main 必須 / front 任意の binary）に置換
#      swaggo は formData の file パラメータを x-www-form-urlencoded の oneOf と空の multipart object に崩す。
#      そのままだと MultipartFormPayload の main / front が生成されず、iOS の写真アップロード
#      （BeGitBackendAPI.uploadPhotos）がビルドエラーになる。
#
# 使い方: sanitize-openapi-for-ios.sh <path-to-openapi.yaml>
set -euo pipefail

target="${1:?usage: sanitize-openapi-for-ios.sh <openapi.yaml>}"
[ -f "$target" ] || { echo "error: file not found: $target" >&2; exit 1; }

perl -0777 -i -pe '
  # 1. type: file -> type: string（インデント維持）
  s/^(\s*)type:\s*file[ \t]*$/${1}type: string/mg;
  # 2. ルート直下の externalDocs ブロック（externalDocs: と 2 スペース字下げの子行）を削除
  s/^externalDocs:\n(?:[ ]{2}\S.*\n)*//mg;
  # 3. photos の requestBody を正しい multipart 定義に置換（swaggo の崩れた形のときだけマッチ）
  s{^(      )requestBody:\n        content:\n          application/x-www-form-urlencoded:\n            schema:\n              oneOf:\n              - title: main\n.*?          multipart/form-data:\n            schema:\n              type: object\n(        description: [^\n]*\n)}{${1}requestBody:\n        required: true\n        content:\n          multipart/form-data:\n            schema:\n              type: object\n              required:\n                - main\n              properties:\n                main:\n                  type: string\n                  format: binary\n                front:\n                  type: string\n                  format: binary\n$2}msg;
' "$target"

echo "✅ iOS 向けに openapi 仕様をサニタイズしました: $target"
