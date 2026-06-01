// Package docs は swag が生成した OpenAPI 仕様と、それを表示する
// Swagger UI ページをバイナリに同梱して配信するためのアセットを提供する。
//
// 仕様の再生成は `make openapi`（swag init --ot json,yaml）で行う。
// docs.go は swag が上書きしない（--ot に go を含めていない）ため手書きで安定。
package docs

import _ "embed"

// SwaggerJSON は OpenAPI 3.1 仕様（JSON）。GET /openapi.json で配信する。
//
//go:embed swagger.json
var SwaggerJSON []byte

// SwaggerYAML は OpenAPI 3.1 仕様（YAML）。GET /openapi.yaml で配信する。
//
//go:embed swagger.yaml
var SwaggerYAML []byte

// SwaggerUIHTML は /openapi.json を読み込む Swagger UI ページ。
// Swagger UI 5 は OpenAPI 3.1 に対応している。アセットは CDN から取得する。
const SwaggerUIHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>BeGit API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: "/openapi.json",
        dom_id: "#swagger-ui",
      });
    };
  </script>
</body>
</html>`
