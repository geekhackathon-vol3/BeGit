//  AuthAPI.swift
//  Backend API のインターフェース・実装
//
//  通信は swift-openapi-generator が openapi.yaml から生成する `Client` / 型
//  (`Components.Schemas.*`) を利用する。手書きの Codable DTO は廃止し、生成型 →
//  ドメインモデルの変換のみをこのファイルで担う（バックエンドと型がズレないようにするため）。

import Foundation
import OpenAPIRuntime
import OpenAPIURLSession
import HTTPTypes

// BeGitバックエンドAPIで発生するエラー
enum BeGitAPIError: LocalizedError {
    case invalidURL         // URL生成に失敗した場合
    case invalidResponse    // HTTPレスポンスをHTTPURLResponseへ変換できなかった場合
    case requestFailed(statusCode: Int, message: String?)   // APIがエラーステータスを返した場合

    var errorDescription: String? {
        switch self {
        case .invalidURL:
            return "API URLが不正です。"
        case .invalidResponse:
            return "APIレスポンスを読み取れませんでした。"
        case let .requestFailed(statusCode, message):
            return message ?? "APIリクエストに失敗しました。status=\(statusCode)"
        }
    }
}

// GitHub OAuth認証を担当するAPIインターフェース
protocol AuthAPI: Sendable {
    // GitHub OAuth認可コードをアクセストークンへ交換する
    func exchangeCode(code: String) async throws -> AuthResponse
}

// リポジトリ関連APIインターフェース
protocol RepositoryAPI: Sendable {
    // 参加中のリポジトリ一覧を取得
    func listRepositories(accessToken: String) async throws -> [Repository]
    // 新しいリポジトリグループを作成
    func createRepository(repoFullName: String, name: String, accessToken: String) async throws -> Repository
    // リポジトリ詳細を取得
    func getRepository(id: Int64, accessToken: String) async throws -> Repository
    // リポジトリのアクティビティ一覧を取得
    func listActivities(repository: Repository, accessToken: String) async throws -> [RepositoryActivity]
    // メンバーへ通知を送信
    func sendNotification(repositoryID: Int64, accessToken: String) async throws
}

// Authorization: Bearer <token> ヘッダを全リクエストへ付与するミドルウェア
private struct AuthMiddleware: ClientMiddleware {
    let token: String

    nonisolated func intercept(
        _ request: HTTPRequest,
        body: HTTPBody?,
        baseURL: URL,
        operationID: String,
        next: @Sendable @concurrent (HTTPRequest, HTTPBody?, URL) async throws -> (HTTPResponse, HTTPBody?)
    ) async throws -> (HTTPResponse, HTTPBody?) {
        var request = request
        request.headerFields[.authorization] = "Bearer \(token)"
        return try await next(request, body, baseURL)
    }
}

// 2xx 以外のレスポンスを BeGitAPIError へ変換して throw するミドルウェア。
// これにより各 API メソッドは成功ケースのみを扱えばよくなる
// （生成コードの型付きエラーケースを毎回 switch する必要がなくなる）。
private struct ErrorThrowingMiddleware: ClientMiddleware {
    nonisolated func intercept(
        _ request: HTTPRequest,
        body: HTTPBody?,
        baseURL: URL,
        operationID: String,
        next: @Sendable @concurrent (HTTPRequest, HTTPBody?, URL) async throws -> (HTTPResponse, HTTPBody?)
    ) async throws -> (HTTPResponse, HTTPBody?) {
        let (response, responseBody) = try await next(request, body, baseURL)
        guard response.status.code >= 300 else {
            return (response, responseBody)
        }

        // エラーボディ（{"error": "..."}）からメッセージを抽出（失敗しても無視）
        var message: String?
        if let responseBody,
           let data = try? await Data(collecting: responseBody, upTo: 64 * 1024) {
            message = (try? JSONDecoder().decode(ErrorResponseDTO.self, from: data))?.error
        }
        throw BeGitAPIError.requestFailed(statusCode: response.status.code, message: message)
    }
}

// BeGitバックエンドとの通信を行う実装
struct BeGitBackendAPI: AuthAPI, RepositoryAPI {
    private let baseURL: URL            // APIエンドポイントのベースURL
    private let session: URLSession     // HTTP通信に利用するURLSession

    nonisolated init(
        baseURL: URL = BeGitBackendAPI.defaultBaseURL,
        session: URLSession = .shared
    ) {
        self.baseURL = baseURL
        self.session = session
    }

    private nonisolated static var defaultBaseURL: URL {
        guard let value = Bundle.main.object(forInfoDictionaryKey: "API_BASE_URL") as? String,
              value.isEmpty == false,
              let url = URL(string: value) else {
            preconditionFailure("API_BASE_URL is not configured")
        }

        return url
    }

    // 生成 Client を構築する。openapi.yaml の servers は相対(/)のため、
    // 実行時の baseURL を serverURL として明示的に上書きする。
    private func makeClient(accessToken: String? = nil) -> Client {
        var middlewares: [any ClientMiddleware] = [ErrorThrowingMiddleware()]
        if let accessToken {
            middlewares.insert(AuthMiddleware(token: accessToken), at: 0)
        }
        return Client(
            serverURL: baseURL,
            transport: URLSessionTransport(configuration: .init(session: session)),
            middlewares: middlewares
        )
    }

    // GitHub OAuthの認可コードをBeGitバックエンドに送信し、認証情報へ変換する
    func exchangeCode(code: String) async throws -> AuthResponse {
        let output = try await makeClient().postAuthGithub(
            .init(body: .json(.Handler_AuthRequest(.init(code: code))))
        )
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        let payload = try ok.body.json
        let user = payload.user

        return AuthResponse(
            accessToken: payload.token ?? "",
            githubUser: GitHubUser(
                id: user?.id ?? 0,
                login: user?.login ?? "",
                avatarURL: user?.avatarUrl.flatMap { URL(string: $0) },
                email: nil
            )
        )
    }

    // ログイン中ユーザーが参加しているリポジトリグループ一覧を取得する
    func listRepositories(accessToken: String) async throws -> [Repository] {
        let output = try await makeClient(accessToken: accessToken).getGroups()
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        return (try ok.body.json.groups ?? []).map { $0.toRepository(members: []) }
    }

    // GitHubリポジトリ名を指定して、新しいBeGitリポジトリグループを作成する
    func createRepository(repoFullName: String, name: String, accessToken: String) async throws -> Repository {
        let output = try await makeClient(accessToken: accessToken).postGroups(
            .init(body: .json(.Handler_CreateGroupRequest(.init(name: name, repoFullName: repoFullName))))
        )
        guard case let .created(created) = output else { throw BeGitAPIError.invalidResponse }
        guard let id = try created.body.json.id else { throw BeGitAPIError.invalidResponse }

        return try await getRepository(id: Int64(id), accessToken: accessToken)
    }

    // 指定したバックエンドIDのリポジトリ詳細を取得する
    func getRepository(id: Int64, accessToken: String) async throws -> Repository {
        let output = try await makeClient(accessToken: accessToken).getGroupsId(
            .init(path: .init(id: Int(id)))
        )
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        return try ok.body.json.toRepository()
    }

    // 指定リポジトリのタイムライン表示用アクティビティ一覧を取得する
    func listActivities(repository: Repository, accessToken: String) async throws -> [RepositoryActivity] {
        guard let backendID = repository.backendID else { return [] }
        let output = try await makeClient(accessToken: accessToken).getGroupsIdPosts(
            .init(path: .init(id: Int(backendID)))
        )
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        return (try ok.body.json.posts ?? []).map { $0.toActivity(fallbackRepository: repository) }
    }

    // 指定リポジトリのメンバーに作業通知を送信する
    func sendNotification(repositoryID: Int64, accessToken: String) async throws {
        let output = try await makeClient(accessToken: accessToken).postGroupsIdNotifications(
            .init(path: .init(id: Int(repositoryID)))
        )
        guard case .created = output else { throw BeGitAPIError.invalidResponse }
    }
}

//  開発・テスト用の認証API
struct MockAuthAPI: AuthAPI {
    func exchangeCode(code: String) async throws -> AuthResponse {
        try await Task.sleep(for: .milliseconds(400))

        return AuthResponse(
            accessToken: "mock_access_token_\(code)",
            githubUser: GitHubUser(
                id: 1,
                login: "octocat",
                avatarURL: URL(string: "https://github.com/octocat.png"),
                email: "octocat@github.com"
            )
        )
    }
}

// エラーレスポンスボディ（{"error": "..."}）
private struct ErrorResponseDTO: Decodable {
    let error: String
}

// MARK: - 生成型 → ドメインモデル変換

private extension Components.Schemas.Handler_GroupJSON {
    // グループ概要 → Repository。repo_full_name が空ならグループ名で代替する。
    func toRepository(members: [RepositoryMember]) -> Repository {
        let fullName = repoFullName ?? ""
        let displayName = fullName.isEmpty ? (name ?? "") : fullName
        return Repository(
            backendID: id.map(Int64.init),
            name: displayName,
            memberCount: members.count,
            members: members
        )
    }
}

private extension Components.Schemas.Handler_GroupDetailJSON {
    // グループ詳細 → Repository（メンバー込み）
    func toRepository() -> Repository {
        let repositoryMembers = (members ?? []).map { $0.toMember() }
        let fullName = repoFullName ?? ""
        let displayName = fullName.isEmpty ? (name ?? "") : fullName
        return Repository(
            backendID: id.map(Int64.init),
            name: displayName,
            memberCount: repositoryMembers.count,
            members: repositoryMembers
        )
    }
}

private extension Components.Schemas.Handler_GroupMemberJSON {
    func toMember() -> RepositoryMember {
        RepositoryMember(
            backendUserID: userId.map(Int64.init),
            login: login ?? "",
            avatarURL: avatarUrl.flatMap { URL(string: $0) }
        )
    }
}

private extension Components.Schemas.Handler_PostFeedJSON {
    // タイムライン投稿 → 画面表示用 RepositoryActivity
    func toActivity(fallbackRepository: Repository) -> RepositoryActivity {
        RepositoryActivity(
            type: activityType,
            title: activityTitle(fallbackRepository: fallbackRepository),
            date: createdAt.flatMap { ISO8601DateFormatter().date(from: $0) } ?? Date(),
            imageName: "begit_timeline_mock",
            author: RepositoryMember(
                backendUserID: userId.map(Int64.init),
                login: login ?? "",
                avatarURL: avatarUrl.flatMap { URL(string: $0) }
            ),
            reaction: reaction
        )
    }

    // 投稿種別からアクティビティ種別へ変換
    private var activityType: RepositoryActivityType {
        switch postType {
        case "pull_request", "pullRequest":
            return .pullRequest
        // "memo" が正。"sorry"/"comment" は旧名称・旧データ互換のため受理
        case "memo", "sorry", "comment":
            return .memo
        default:
            return .commit
        }
    }

    // アクティビティ種別に応じたリアクションを生成
    private var reaction: RepositoryReaction? {
        switch activityType {
        case .commit:
            return .check
        case .pullRequest:
            return .heart
        case .memo:
            return .sorry
        }
    }

    // タイムラインに表示するタイトルを生成
    private func activityTitle(fallbackRepository: Repository) -> String {
        if let latestCommitMessage, latestCommitMessage.isEmpty == false {
            return latestCommitMessage
        }
        if let body, body.isEmpty == false {
            return body
        }
        let commits = commitCount ?? 0
        if commits > 0 {
            return "\(commits) commits in \(repoFullName ?? fallbackRepository.name)"
        }
        return status ?? "No activity yet"
    }
}
