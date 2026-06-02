//  AuthAPI.swift
//  Backend APIのインターフェース・実装

import Foundation

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

// BeGitバックエンドとの通信を行う実装
struct BeGitBackendAPI: AuthAPI, RepositoryAPI {
    private let baseURL: URL            // APIエンドポイントのベースURL
    private let session: URLSession     // HTTP通信に利用するURLSession
    private let decoder: JSONDecoder    // レスポンスJSONデコード用
    private let encoder: JSONEncoder    // リクエストJSONエンコード用

    nonisolated init(
        baseURL: URL = BeGitBackendAPI.defaultBaseURL,
        session: URLSession = .shared
    ) {
        self.baseURL = baseURL
        self.session = session
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        self.decoder = decoder
        let encoder = JSONEncoder()
        encoder.keyEncodingStrategy = .convertToSnakeCase
        self.encoder = encoder
    }

    private nonisolated static var defaultBaseURL: URL {
        guard let value = Bundle.main.object(forInfoDictionaryKey: "API_BASE_URL") as? String,
              value.isEmpty == false,
              let url = URL(string: value) else {
            preconditionFailure("API_BASE_URL is not configured")
        }

        return url
    }

    // GitHub OAuthの認可コードをBeGitバックエンドに送信し、認証情報へ変換する
    func exchangeCode(code: String) async throws -> AuthResponse {
        let response: AuthResponseDTO = try await request(
            path: "/auth/github",
            method: "POST",
            body: AuthRequestDTO(code: code)
        )

        return AuthResponse(
            accessToken: response.token,
            githubUser: GitHubUser(
                id: Int(response.user.id),
                login: response.user.login,
                avatarURL: URL(string: response.user.avatarUrl),
                email: nil
            )
        )
    }

    // ログイン中ユーザーが参加しているリポジトリグループ一覧を取得する
    func listRepositories(accessToken: String) async throws -> [Repository] {
        let response: GroupsResponseDTO = try await request(
            path: "/groups",
            method: "GET",
            accessToken: accessToken
        )

        return response.groups.map { $0.repository(members: []) }
    }

    // GitHubリポジトリ名を指定して、新しいBeGitリポジトリグループを作成する
    func createRepository(repoFullName: String, name: String, accessToken: String) async throws -> Repository {
        let group: GroupDTO = try await request(
            path: "/groups",
            method: "POST",
            accessToken: accessToken,
            body: CreateGroupRequestDTO(repoFullName: repoFullName, name: name)
        )

        return try await getRepository(id: group.id, accessToken: accessToken)
    }

    // 指定したバックエンドIDのリポジトリ詳細を取得する
    func getRepository(id: Int64, accessToken: String) async throws -> Repository {
        let detail: GroupDetailDTO = try await request(
            path: "/groups/\(id)",
            method: "GET",
            accessToken: accessToken
        )

        return detail.repository()
    }

    // 指定リポジトリのタイムライン表示用アクティビティ一覧を取得する
    func listActivities(repository: Repository, accessToken: String) async throws -> [RepositoryActivity] {
        guard let backendID = repository.backendID else { return [] }
        let response: PostsResponseDTO = try await request(
            path: "/groups/\(backendID)/posts",
            method: "GET",
            accessToken: accessToken
        )

        return response.posts.map { $0.activity(fallbackRepository: repository) }
    }

    // 指定リポジトリのメンバーに作業通知を送信する
    func sendNotification(repositoryID: Int64, accessToken: String) async throws {
        let _: NotificationDTO = try await request(
            path: "/groups/\(repositoryID)/notifications",
            method: "POST",
            accessToken: accessToken,
            body: EmptyRequestDTO()
        )
    }

    // 共通APIリクエスト処理
    // リクエスト送信・レスポンス検証・JSONデコードを行う
    private func request<Response: Decodable>(
        path: String,
        method: String,
        accessToken: String? = nil
    ) async throws -> Response {
        try await request(path: path, method: method, accessToken: accessToken, body: Optional<EmptyRequestDTO>.none)
    }

    private func request<Response: Decodable, Body: Encodable>(
        path: String,
        method: String,
        accessToken: String? = nil,
        body: Body? = nil
    ) async throws -> Response {
        guard let url = URL(string: path, relativeTo: baseURL) else {
            throw BeGitAPIError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let accessToken {
            request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        }
        if let body {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = try encoder.encode(body)
        }

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw BeGitAPIError.invalidResponse
        }

        guard (200..<300).contains(httpResponse.statusCode) else {
            let errorResponse = try? decoder.decode(ErrorResponseDTO.self, from: data)
            throw BeGitAPIError.requestFailed(statusCode: httpResponse.statusCode, message: errorResponse?.error)
        }

        return try decoder.decode(Response.self, from: data)
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

// GitHub認証リクエストDTO
private struct AuthRequestDTO: Encodable {
    let code: String
}

private struct EmptyRequestDTO: Encodable {}

// グループ作成リクエストDTO
private struct CreateGroupRequestDTO: Encodable {
    let repoFullName: String    // GitHubリポジトリ名 (owner/repository)
    let name: String            // アプリ内表示名
}

// 認証APIレスポンスDTO
private struct AuthResponseDTO: Decodable {
    let user: UserDTO   // GitHubユーザー情報
    let token: String   // BeGitアクセストークン
}

// グループ情報DTO
private struct UserDTO: Decodable {
    let id: Int64           // GitHubユーザーID
    let login: String       // GitHubユーザー名
    let avatarUrl: String   // アバター画像URL
    let name: String        // 表示名
}

private struct GroupsResponseDTO: Decodable {
    let groups: [GroupDTO]
}

private struct GroupDTO: Decodable {
    let id: Int64               // バックエンド管理ID
    let name: String            // グループ名
    let repoFullName: String    // GitHubリポジトリ名
    let avatarUrl: String       // リポジトリアイコンURL

    func repository(members: [RepositoryMember]) -> Repository {
        Repository(
            backendID: id,
            name: repoFullName.isEmpty ? name : repoFullName,
            memberCount: members.count,
            members: members
        )
    }
}

private struct GroupDetailDTO: Decodable {
    let id: Int64               // バックエンド管理ID
    let name: String            // グループ名
    let repoFullName: String    // GitHubリポジトリ名
    let avatarUrl: String       // リポジトリアイコンURL
    let members: [GroupMemberDTO] // 所属メンバー一覧

    func repository() -> Repository {
        let repositoryMembers = members.map { $0.member() }
        return Repository(
            backendID: id,
            name: repoFullName.isEmpty ? name : repoFullName,
            memberCount: repositoryMembers.count,
            members: repositoryMembers
        )
    }
}

private struct GroupMemberDTO: Decodable {
    let userId: Int64
    let login: String
    let avatarUrl: String
    let role: String

    func member() -> RepositoryMember {
        RepositoryMember(
            backendUserID: userId,
            login: login,
            avatarURL: URL(string: avatarUrl)
        )
    }
}

private struct PostsResponseDTO: Decodable {
    let posts: [PostDTO]
}

// タイムライン投稿DTO
private struct PostDTO: Decodable {
    let id: Int64               // 投稿ID
    let userId: Int64           // 投稿ユーザーID
    let postType: String        // 投稿種別(commit / pull_request / memo)
    let body: String?           // 投稿本文
    let repoFullName: String?   // リポジトリ名
    let commitCount: Int        // コミット数
    let additions: Int          // 追加行数
    let deletions: Int          // 削除行数
    let latestCommitMessage: String?    // 最新コミットメッセージ
    let status: String?         // 状態メッセージ
    let createdAt: String       // 投稿作成日時(ISO8601)
    let login: String           // 投稿者ユーザー名
    let avatarUrl: String       // 投稿者アバターURL

    // 投稿DTOを画面表示用のRepositoryActivityへ変換
    func activity(fallbackRepository: Repository) -> RepositoryActivity {
        RepositoryActivity(
            type: activityType,
            title: activityTitle(fallbackRepository: fallbackRepository),
            date: ISO8601DateFormatter().date(from: createdAt) ?? Date(),
            imageName: "begit_timeline_mock",
            author: RepositoryMember(
                backendUserID: userId,
                login: login,
                avatarURL: URL(string: avatarUrl)
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
        if commitCount > 0 {
            return "\(commitCount) commits in \(repoFullName ?? fallbackRepository.name)"
        }
        return status ?? "No activity yet"
    }
}

private struct NotificationDTO: Decodable {
    let id: Int64
    let sprintId: Int64
    let sentAt: String
}

private struct ErrorResponseDTO: Decodable {
    let error: String
}
