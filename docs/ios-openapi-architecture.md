# iOS の OpenAPI 通信レイヤー — 全体像

「`openapi.yaml` から型と API クライアントを自動生成し、人間は“翻訳層”だけ書く」状態の
仕組みと、日々どう運用するかをまとめる。手書きの `URLSession` + `Codable` は廃止済み。

> セットアップ手順（パッケージ追加など）は [ios-openapi-setup.md](./ios-openapi-setup.md) を参照。
> ここは「出来上がったものがどう動くか」を説明する。

---

## 3 層構造

```
①  openapi.yaml                       ← バックエンドが配る API 設計図（唯一の正 / source of truth）
        │
        │  make openapi-sync で backend から同期 → sanitize（後述）
        │  ビルド時に swift-openapi-generator プラグインが自動生成
        ▼
②  生成コード（git に無い。ビルドのたびに DerivedData へ出力される）
    ・Client … getGroups() / postAuthGithub() など「実際に HTTP を投げる」メソッド
    ・Components.Schemas.Handler_* … JSON に対応する struct（avatarUrl 等 camelCase）
        │
        │  これを呼び出して使う
        ▼
③  手書きの薄い“翻訳層”（ios/BeGit/BeGit/Services/）
    ・BackendAPI.swift          … アプリが必要とする API の形（プロトコル）＋エラー型＋Mock
    ・BeGitBackendAPI.swift     … ②を呼び、成功結果を取り出す本実装＋認証/エラーのミドルウェア
    ・BackendSchemaMapping.swift … 生成型 → ドメインモデル(Repository 等) への変換
        │
        ▼
   ViewModel（プロトコルにのみ依存。実装が変わっても無変更）
```

ポイント: **URL 組み立て・送信・JSON デコードは ② が持つ**。③ には書かない。
だから「`BeGitBackendAPI.swift` を見ても通信の中身が無い」のが正常。実体は生成コード側にある。

---

## 各ファイルの責務（③ の中身）

| ファイル | 役割 |
|---|---|
| [Services/BackendAPI.swift](../ios/BeGit/BeGit/Services/BackendAPI.swift) | `AuthAPI` / `RepositoryAPI` プロトコル、`BeGitAPIError`、`MockAuthAPI`。アプリが依存する「契約」。 |
| [Services/BeGitBackendAPI.swift](../ios/BeGit/BeGit/Services/BeGitBackendAPI.swift) | 生成 `Client` を使う本実装。`AuthMiddleware`(Bearer 付与)、`ErrorThrowingMiddleware`(非 2xx をエラー化)、6 エンドポイントの呼び出し。 |
| [Services/BackendSchemaMapping.swift](../ios/BeGit/BeGit/Services/BackendSchemaMapping.swift) | 生成型 `Components.Schemas.Handler_*` → `Repository` / `RepositoryMember` / `RepositoryActivity` への変換。 |

各メソッドの形は統一されている:

```swift
// 生成 Client を呼ぶ → 成功ケースを取り出す → ドメイン型へ翻訳
let output = try await makeClient(accessToken: token).getGroups()
guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
return (try ok.body.json.groups ?? []).map { $0.toRepository(members: []) }
```

成功ステータスの違いに注意: 作成系（`POST /groups` と `POST /groups/{id}/notifications`）は
**201 = `.created`**、それ以外は **200 = `.ok`**。生成 enum がこれを区別する。

---

## アプリが使うエンドポイント

`openapi-generator-config.yaml` の `filter` で、アプリが実際に使う 6 つだけを生成している
（写真投稿などは未使用なので生成対象外）。

`POST /auth/github` / `GET /groups` / `POST /groups` / `GET /groups/{id}` /
`GET /groups/{id}/posts` / `POST /groups/{id}/notifications`

> GitHub を直接叩く [GitHubRepositoryAPI.swift](../ios/BeGit/BeGit/Services/GitHubRepositoryAPI.swift)
> は BeGit バックエンドではないので、この仕組みの対象外（従来どおり）。

---

## なぜ「OpenAPI 準拠」なのか（手書き DTO との違い）

- 以前: バックエンドのレスポンス構造を iOS でも手書きの DTO として写経していた。
  バックエンドが `post_type` を `comment`→`memo` に変えても iOS は気づけず、文字列ベタ書きで追従していた（ズレの温床）。
- 現在: `openapi.yaml` が唯一の正。スキーマが変われば**生成型が変わり、翻訳層がコンパイルエラー**になって気づける。型の二重管理が無くなる。

---

## 日々の運用

バックエンドの API が変わったら、フロントは 2 ステップ:

```bash
make openapi-sync   # backend/docs/swagger.yaml を iOS へ同期 + サニタイズ（リポジトリルートで）
# → Xcode でリビルド（⌘B）
```

これで生成型が最新化され、必要なら翻訳層 (`BackendSchemaMapping.swift` 等) を直す。

### サニタイズについて
`make openapi-sync` は同期後に [scripts/sanitize-openapi-for-ios.sh](../scripts/sanitize-openapi-for-ios.sh)
を実行する。swaggo(`--v3.1`) が出力する仕様には OpenAPIKit がパースできない不正な
OpenAPI 3.1 構造（`type: file`、空 url の `externalDocs`）が含まれるため、iOS 取り込み用に補正している。
（本来はバックエンド側で直すのが望ましい論点。詳細はスクリプト冒頭コメント参照）
