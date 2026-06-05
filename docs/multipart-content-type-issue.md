# swift-openapi-generator で multipart 画像アップロードしたら 422 が返ってきた話

## TL;DR

- swift-openapi-runtime は `format: binary` のマルチパートパートに `Content-Type: text/plain` を付ける（schema の `type: string` に引きずられる）
- バックエンドがその `text/plain` を信じて画像バリデーションに失敗していた
- OpenAPI spec に `encoding` を追加するか、バックエンドで `http.DetectContentType` を使ってバイト列から MIME を判定すれば解決する

---

## 背景

iOS アプリ（Swift）から Go バックエンドへ、マルチパートフォームで画像をアップロードする機能を実装していた。

- iOS クライアント: [swift-openapi-generator](https://github.com/apple/swift-openapi-generator) でコード自動生成
- バックエンド: Go + Gin

OpenAPI spec は以下のように定義した。

```yaml
/groups/{id}/posts/{postId}/photos:
  post:
    requestBody:
      required: true
      content:
        multipart/form-data:
          schema:
            type: object
            required:
              - main
            properties:
              main:
                type: string
                format: binary
              front:
                type: string
                format: binary
```

---

## 起きた問題

iOS からアップロードリクエストを送ると、バックエンドから `422 Unprocessable Entity` が返ってきた。

```
Upload failed: requestFailed(statusCode: 422, message: Optional("invalid photo"))
```

---

## 原因調査

### バックエンド側の検証ロジック

バックエンドでは受け取ったファイルの `Content-Type` をチェックして、許可されたフォーマット（JPEG / PNG / HEIC）かどうかを検証していた。

```go
var allowedPhotoContentTypes = map[string]string{
    "image/jpeg": "jpg",
    "image/png":  "png",
    "image/heic": "heic",
}

func validatePhoto(f *UploadFile) (string, error) {
    if _, ok := allowedPhotoContentTypes[f.ContentType]; !ok {
        return "", fmt.Errorf("%w: unsupported content type %q", ErrValidation, f.ContentType)
    }
    // ...
}
```

`f.ContentType` は multipart パートのヘッダーから取得していた。

```go
func contentTypeOf(header *multipart.FileHeader) string {
    return header.Header.Get("Content-Type")
}
```

### エラーメッセージに情報を足して犯人を特定する

ログだけでは原因が掴めなかったので、エラーメッセージに「実際に受け取った Content-Type」と「データ先頭バイト」を埋め込んでクライアント側のログで見えるようにした。

```go
// 一時的なデバッグ：弾かれた content type とデータ先頭バイトを返す
case errors.Is(err, service.ErrValidation):
    prefix := fmt.Sprintf("%x", main.Data[:min(24, len(main.Data))])
    respondError(c, http.StatusUnprocessableEntity,
        fmt.Sprintf("invalid photo: %s (mainLen=%d prefix=%s)", err.Error(), len(main.Data), prefix))
```

> Cloudflare Workers Container（Durable Object Container）上で動く Go の `log.Printf` は、Workers の Observability ログにも `wrangler tail` にも流れてこなかった。コンテナの stdout は別系統のため。**エラーレスポンスのボディに情報を詰める**のが、この構成では一番確実な観測手段だった。

すると、こんなメッセージが返ってきた。

```
invalid photo: validation error: unsupported content type "text/plain"
  (mainLen=248437 prefix=ffd8ffe000104a46494600010100004800480000ffe1008c)
```

### 先頭バイトを読む

`prefix` をデコードすると：

```
ff d8 ff e0       → JPEG の SOI + APP0 マーカー（完全に正しい JPEG）
00 10             → セグメント長 16
4a 46 49 46 00    → "JFIF\0"
```

**データは完璧に正しい JPEG だった。** 248KB のれっきとした画像が届いている。

それなのに `Content-Type` が `text/plain` として検証に渡っていた。つまり、

- iOS は正しい JPEG バイナリを送っている
- しかし multipart パートの `Content-Type` ヘッダーには `text/plain` が入っている

### なぜ text/plain が付くのか

最初は「swift-openapi-runtime は `Content-Type` を付けないのだろう」と推測していたが、実際は違った。**`text/plain` を明示的に付けていた。**

理由は OpenAPI spec の型定義にある。`format: binary` であっても、schema 上は `type: string` だ。swift-openapi-generator は「string 型のパートだからデフォルトの `text/plain`」としてエンコードする。`encoding` セクションで上書きしない限り、バイナリであることはランタイムに伝わらない。

結果として、バックエンドが `header.Header.Get("Content-Type")` で `text/plain` を受け取り、許可リスト（image/*）に無いとして弾いていた。

---

## 解決策

### 解決策 A（正道）: OpenAPI spec に `encoding` を追加する

OpenAPI 3.x では `encoding` セクションで各フィールドの `Content-Type` を指定できる。これを追加すると、swift-openapi-runtime がパートヘッダーに正しい `Content-Type` を付与する。

```yaml
requestBody:
  required: true
  content:
    multipart/form-data:
      schema:
        type: object
        required:
          - main
        properties:
          main:
            type: string
            format: binary
          front:
            type: string
            format: binary
      encoding:
        main:
          contentType: image/jpeg, image/png, image/heic
        front:
          contentType: image/jpeg, image/png, image/heic
```

### 解決策 B（堅牢な実装）: バックエンドでバイト列から判定する

そもそも画像アップロードの検証で、クライアント申告の `Content-Type` を信用するのは危うい。実体のバイト列から MIME を判定する方が確実だ。

Go の標準ライブラリ `net/http` には `DetectContentType` がある。先頭 512 バイトを読み、[sniff アルゴリズム](https://mimesniff.spec.whatwg.org/)で MIME タイプを推測する。JPEG なら先頭 `FF D8 FF` から `image/jpeg` と判定される。

```go
// swift-openapi-runtime は format:binary パートに Content-Type: text/plain を付ける
// （schema type が string のため）。multipart ヘッダは信頼せず、バイト列から MIME を判定する。
func contentTypeOf(header *multipart.FileHeader, data []byte) string {
    if detected := http.DetectContentType(data); strings.HasPrefix(detected, "image/") {
        return detected
    }
    // Go の sniff が判定できない画像形式（HEIC 等）向けに、ヘッダが image/* ならそれを使う。
    if headerCT := header.Header.Get("Content-Type"); strings.HasPrefix(headerCT, "image/") {
        return headerCT
    }
    return http.DetectContentType(data)
}
```

> 注意: `http.DetectContentType` は HEIC を判定できない（Go の sniff リストに無い）。HEIC を受け付けるなら、`....ftypheic` の box を自前で見るか、解決策 A で `encoding` を効かせる必要がある。今回の iOS は `UIImage.jpegData()` で JPEG を送るため、バイト判定で十分だった。

---

## どちらを選ぶか

| | 解決策 A（spec 修正） | 解決策 B（バックエンド）|
|---|---|---|
| 本質的な修正 | ✅ spec が正確になる | △ クライアントの不備を補う |
| 適用範囲 | iOS のみ | どのクライアントにも効く |
| 手間 | コード再生成が必要 | 1 関数の変更だけ |
| 堅牢性 | クライアント依存 | クライアントに依存しない |
| HEIC 対応 | ✅ encoding で明示できる | ❌ 別途 box 解析が必要 |

今回はバックエンドの修正（解決策 B）を採用した。クライアント申告に依存せず、バイト列という「事実」で判定する方が、アップロード検証としては堅牢だと判断したため。理想的には両方適用するのがベスト。

---

## ハマりどころの教訓

1. **`text/plain` は「Content-Type 未設定」ではなく「string 型のデフォルト」だった。** 最初の推測（ヘッダーを付けない）は誤りで、実際は誤ったヘッダーが付いていた。憶測で原因を決めず、実データを観測することが重要。

2. **コンテナの stdout ログが見えない環境では、エラーレスポンスボディにデバッグ情報を詰める。** `log.Printf` が届かないなら、確実に手元まで返ってくる経路（レスポンス）に情報を載せる。先頭バイトの hex を返したことで一発で原因が確定した。

3. **`format: binary` は `type: string` のサブ分類でしかない。** multipart でバイナリを正しく送るには `encoding` で MIME を明示する必要がある。これは TypeScript / Python など他の OpenAPI クライアントでも同様に踏みうる罠。

---

## まとめ

swift-openapi-generator が画像パートに `text/plain` を付けるのは「ライブラリの不備」ではなく、**spec が `type: string` としか言っていないから**だ。`format: binary` だけでは MIME は伝わらず、`encoding` セクションで明示する必要がある。

バックエンド側でバイト列から MIME を判定するフォールバックを入れておくと、クライアント実装に依存しない堅牢な API になる。
