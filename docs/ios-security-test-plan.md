# iOS セキュリティ攻撃テスト手順

対象: `ios/BeGit/BeGit`。自分たちの開発環境、テストアカウント、ステージング API で実施する。実在する第三者のリポジトリ、Webhook、端末には送らない。

## 1. 自動テスト

`BeGitSecurityTests/SecurityAttackTests.swift` に攻撃入力と安全側の期待結果を記述した。Xcode の `BeGit` スキームで iOS Simulator を選び、`Product > Test`（⌘U）を実行する。CLI の例:

```bash
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer \
  xcodebuild test \
  -project ios/BeGit/BeGit.xcodeproj \
  -scheme BeGit \
  -destination 'platform=iOS Simulator,name=iPhone 17' \
  -only-testing:BeGitSecurityTests
```

Simulator 名は `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcrun simctl list devices available` の実際の名前に置き換える。テストコードはネットワークに接続せず、本物の Keychain トークンを参照しない。`savedGitHubAppInstallationID` の元の値は各テスト後に戻す。

| ID | 攻撃入力 | 安全側の合格条件 | 実行結果 |
| --- | --- | --- | --- |
| A1 | OAuth callback の scheme/host/state を変更 | 認証コードを交換しない | 合格 |
| A2 | `state`/`code` の重複、余計なパス | callback を拒否 | 合格 |
| A3 | 未ログインで `begit://github-app-setup?installation_id=...` | ID を保存しない | 合格 |
| A4 | installation ID の重複、余計なパス | callback を拒否 | 合格 |
| A5 | 未知の通知 type、不正数値、整数オーバーフロー | 通知を無視 | 合格 |
| A6 | 負数または 0 の `group_id`/`post_id`/`sprint_id` | 画面遷移しない | 合格 |

正常な OAuth callback、ログイン中の GitHub App callback、正数 ID の通知を受け入れる 3 件も追加した。2026-10-29 に iPhone 17 / iOS 26.5 Simulator で実行し、**9 件すべて成功**した。

## 2. 端末・統合テスト

テスト用のアカウント A/B と、A だけが参加するテスト用グループを用意する。各項目で「操作前の状態」「リクエスト/レスポンスのステータス」「操作後の表示と保存データ」を記録する。認証ヘッダー、OAuth code、FCM token、Webhook URL の値はログに残さず伏せる。

| ID | 攻撃と手順 | 合格条件 |
| --- | --- | --- |
| M1 認可越え | A でログインし、B 専用の group/post/notification ID に置き換えた API リクエストをステージングへ送る。GET と更新系を 1 件ずつ試す。 | サーバーが 403/404 を返し、A の画面に B の情報が出ず、B のデータが変化しない。通知の ID だけで権限が付かないことを確認する。 |
| M2 失効トークン | A でログアウトした後、ログアウト前の Bearer token をステージング API に 1 回送る。アプリを再起動する。 | API が 401 を返し、再起動後も未ログイン。Keychain に利用可能なトークンが残らない。 |
| M3 通信の盗聴・改ざん | 検査用プロキシを設定し、無効な証明書を提示した HTTPS エンドポイントへテストビルドの API 接続を向ける。HTTP の `API_BASE_URL` も別ビルドで試す。 | 無効証明書は接続失敗。HTTP への認証情報送信は起きない。プロキシには本番トークンを流さない。 |
| M4 URL スキーム偽装 | Simulator で `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcrun simctl openurl booted 'begit://github-app-setup?installation_id=987654321'` を未ログイン・ログイン済みの両方で実行。続けて重複パラメータ、負数、余計なパスを試す。 | 意図しない ID が保存されない。GitHub App の連携開始と戻りの相関が取れない URL は拒否される。 |
| M5 Push 入力偽装 | テスト環境から未知の `type`、巨大 ID、負数 ID、別グループ ID の FCM data を送る。通知をタップする。 | クラッシュせず、無効 payload は遷移しない。別グループのコンテンツは API 側でも拒否される。 |
| M6 保存データ | テストアカウントでログインし、アプリの UserDefaults、Keychain、バックアップ対象ファイルを開発用端末で確認する。 | アクセストークンは Keychain のみにあり、UserDefaults/ファイル/バックアップに平文で残らない。必要な Keychain accessibility と移行可否が仕様に合う。 |
| M7 機密ログ | ログイン、Push 登録、ログアウトを行い、Xcode コンソールと端末ログを確認する。 | OAuth code、Bearer token、APNs/FCM token、Webhook URL がログに表示されない。 |
| M8 画像・Webhook | 撮影画像を投稿し、保存先画像の EXIF 位置情報を確認する。Webhook URL には `http://`、ループバック、プライベート IP、クラウドのメタデータ IP をステージングで入力する。 | 位置情報を不要に公開しない。Webhook 宛先の危険な URL はサーバーが拒否し、内部ネットワークへ送信しない。 |

`M1`、`M5`、`M8` の最終的な認可・宛先検証はバックエンドで判定する。iOS の入力検証だけを合格根拠にしない。

## 3. 現時点で確認できた要調査箇所

- `App/AuthState.swift`: ログイン、URL のパス、重複パラメータは検査する。GitHub App 連携開始時の nonce との照合はまだないため、ログイン中に任意の正数 installation ID を含む URL を受け入れる可能性がある。`M4` で連携対象の取り違えを確認する。
- `Services/GitHubOAuthManager.swift`: 重複 `state`/`code` と余計なパスは拒否する。エラー表示には期待 state や通信エラー詳細が含まれる可能性がある。
- `App/AppDelegate.swift`: APNs device token と FCM registration token を `print` している。リリースビルドでの露出を確認する。
- `Services/KeychainManager.swift`: `kSecAttrAccessible` を明示していない。利用条件と端末間移行要件を決めてから属性を指定する。
- `Services/GitHubRepositoryAPI.swift`: iOS から GitHub API に Bearer token を直接送信している。設計書の「GitHub API はバックエンド経由」との整合、およびトークンの権限範囲を確認する。

基準: [OWASP MASVS](https://mas.owasp.org/MASVS/03-Using_the_MASVS/)、[Apple の Keychain accessibility](https://developer.apple.com/documentation/security/restricting-keychain-item-accessibility)、[Apple の安全なネットワーク接続](https://developer.apple.com/documentation/security/preventing-insecure-network-connections)。
