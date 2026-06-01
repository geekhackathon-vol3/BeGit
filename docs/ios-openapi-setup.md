# iOS フロント向け OpenAPI セットアップガイド

バックエンドの API を **手書きの `Codable` なしで** Swift から型安全に叩くためのガイドです。
OpenAPI を触ったことがなくても大丈夫なように、仕組みから順に説明します。

---

## TL;DR

1. Xcode に SwiftPM パッケージを 3 つ追加する
2. `ios/BeGit/BeGit/` に `openapi.yaml` と `openapi-generator-config.yaml` を置く
3. ターゲットに **OpenAPIGenerator プラグイン**を紐付ける
4. ビルドすると `Client` と型（`Components.Schemas.*`）が**自動生成**される
5. API が変わったら `make openapi-sync` → Xcode でリビルドするだけ

---

## そもそも OpenAPI とは？（30 秒で）

OpenAPI は「この API はどんなエンドポイントがあって、どんな JSON を受け取って返すか」を
**機械が読める形式（YAML/JSON）で書いた仕様書**です。

BeGit ではバックエンド（Go）のコードに書かれた注釈から、この仕様書を自動生成しています。

```
Go のコード (backend/internal/handler/*.go)
        │  make openapi
        ▼
  openapi.yaml   ← 「API の設計図」。これが唯一の真実(source of truth)
        │  swift-openapi-generator
        ▼
  Swift の型 + APIクライアント (ビルド時に自動生成)
```

ポイントは **「Swift の型を手で書かない」** こと。設計図(`openapi.yaml`)から自動で
`struct` や API 呼び出しメソッドが生えてくるので、バックエンドと型がズレません。

仕様の中身はブラウザでも確認できます（バックエンドが動いていれば）:

- Swagger UI（人間用の見やすい画面）: `http://localhost:8080/docs`
- 仕様ファイルそのもの: `http://localhost:8080/openapi.yaml`

---

## なぜ Apple 公式 swift-openapi-generator を使うのか

- Apple 純正で iOS/Swift との相性が良い
- **ビルド時に生成**するので、生成コードを git にコミットする必要がない
  （= 生成物のコンフリクトや「更新し忘れ」が起きない）
- `async/await` ベースのモダンな API

---

## セットアップ手順

### 1. SwiftPM パッケージを追加

Xcode で **File → Add Package Dependencies...** から以下 3 つを追加します。

| パッケージ URL | 役割 |
|---|---|
| `https://github.com/apple/swift-openapi-generator` | コードを生成するプラグイン |
| `https://github.com/apple/swift-openapi-runtime` | 生成コードが使う共通ランタイム |
| `https://github.com/apple/swift-openapi-urlsession` | URLSession で通信するトランスポート |

追加時にターゲット（`BeGit`）へリンクするライブラリとして
**`OpenAPIRuntime`** と **`OpenAPIURLSession`** を選びます
（`swift-openapi-generator` 本体はプラグインなのでリンク不要）。

> 必要環境: Xcode 15 以上 / Swift 5.9 以上

### 2. 仕様ファイルと設定ファイルを配置

`ios/BeGit/BeGit/` 直下に 2 つのファイルを置きます。

**`openapi.yaml`** … バックエンドの仕様。手で作らず、リポジトリルートで:

```bash
make openapi-sync
```

を実行すると `backend/docs/swagger.yaml` がここへコピーされます。

**`openapi-generator-config.yaml`** … 生成内容の設定。以下をそのまま作成:

```yaml
generate:
  - types    # JSON のモデル(struct/enum)を生成
  - client   # API を叩くメソッドを生成
accessModifier: public
```

> この 2 ファイルは **ターゲットの「メンバーシップ」に含める**必要があります。
> Xcode の File Inspector で `BeGit` ターゲットにチェックが入っていることを確認してください。

### 3. ターゲットにプラグインを紐付ける

1. プロジェクト設定 → ターゲット `BeGit` → **Build Phases**
2. **Run Build Tool Plug-ins** に `OpenAPIGenerator` を追加

これで **ビルドのたびに `openapi.yaml` から自動でコードが生成**されます。
生成物は DerivedData 内に置かれ、git には入りません。

### 4. ビルドして使う

ビルドが通れば、以下のように使えます。

```swift
import OpenAPIRuntime
import OpenAPIURLSession

// クライアントを作る
let client = Client(
    serverURL: URL(string: "http://localhost:8080")!,
    transport: URLSessionTransport()
)

// API を呼ぶ（メソッド名は openapi.yaml の operationId から生成される）
let response = try await client.getGroups()

switch response {
case .ok(let ok):
    let groups = try ok.body.json   // 型付きの配列が返る
    print(groups)
case .undocumented(let statusCode, _):
    print("想定外のステータス: \(statusCode)")
}
```

> メソッド名や型名は `openapi.yaml` の内容から決まります。
> 正確な名前は Swagger UI（`/docs`）や、ビルド後のコード補完で確認してください。

---

## 認証ヘッダ（Bearer トークン）の付け方

BeGit の API は `Authorization: Bearer <token>` が必要です。
毎回手で付けるのは面倒なので、**ミドルウェア(ClientMiddleware)** でまとめて付与します。

```swift
import OpenAPIRuntime
import HTTPTypes

struct AuthMiddleware: ClientMiddleware {
    let token: String

    func intercept(
        _ request: HTTPRequest,
        body: HTTPBody?,
        baseURL: URL,
        operationID: String,
        next: (HTTPRequest, HTTPBody?, URL) async throws -> (HTTPResponse, HTTPBody?)
    ) async throws -> (HTTPResponse, HTTPBody?) {
        var request = request
        request.headerFields[.authorization] = "Bearer \(token)"
        return try await next(request, body, baseURL)
    }
}

let client = Client(
    serverURL: URL(string: "http://localhost:8080")!,
    transport: URLSessionTransport(),
    middlewares: [AuthMiddleware(token: "dev_alice")]   // dev トークンの取り方は dev-api-guide.md 参照
)
```

> dev 環境のトークン取得は [`dev-api-guide.md`](./dev-api-guide.md) を参照。

---

## 日々の運用（重要）

バックエンドの API が変わったら、フロント側は **この 2 ステップだけ**:

```bash
# 1. 最新仕様を iOS へ同期（リポジトリルートで）
make openapi-sync

# 2. Xcode でリビルド（⌘B）
```

これで Swift の型が自動で最新化されます。
**`openapi.yaml` を手で編集しないでください**（次の `make openapi-sync` で上書きされます）。
仕様を変えたい場合はバックエンド班に依頼してください。

---

## よくあるハマりどころ

| 症状 | 原因 / 対処 |
|---|---|
| ビルドで「No such module 'OpenAPIRuntime'」 | パッケージ追加時にターゲットへ `OpenAPIRuntime` / `OpenAPIURLSession` をリンクし忘れ |
| 生成された型が見つからない / 古い | `openapi.yaml` がターゲットメンバーシップに入っていない、またはリビルドしていない |
| メソッド名が分からない | `/docs`（Swagger UI）で operationId を確認、またはビルド後にコード補完で探す |
| プラグインの実行許可を聞かれる | Xcode の「Trust & Enable」を承認（初回のみ） |
| 401 が返る | `Authorization: Bearer <token>` が付いていない（上の AuthMiddleware を確認） |

---

## 参考リンク

- swift-openapi-generator: https://github.com/apple/swift-openapi-generator
- 公式チュートリアル: https://swiftpackageindex.com/apple/swift-openapi-generator/documentation
- BeGit dev API ガイド: [`dev-api-guide.md`](./dev-api-guide.md)
