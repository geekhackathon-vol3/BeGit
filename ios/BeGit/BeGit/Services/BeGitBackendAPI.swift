//  BeGitBackendAPI.swift
//  AuthAPI / RepositoryAPI の本実装。openapi.yaml から生成された Client を呼び、
//  結果をドメイン型へ変換する（変換は BackendSchemaMapping.swift）。
//  全体像は docs/ios-openapi-architecture.md を参照。

import Foundation
import OpenAPIRuntime
import OpenAPIURLSession
import HTTPTypes
import BeGitOpenAPIClient

// Authorization: Bearer を全リクエストへ付与する。
private struct AuthMiddleware: ClientMiddleware {
    let token: String

    // @Sendable @concurrent は ClientMiddleware 要件と一致させるため必須（外すとビルド不可）。
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

// 2xx 以外を BeGitAPIError へ変換して throw する（各メソッドは成功ケースのみ扱えばよくなる）。
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

        if response.status.code == 401 {
            throw BeGitAPIError.authenticationRequired
        }

        var message: String?
        if let responseBody,
           let data = try? await Data(collecting: responseBody, upTo: 64 * 1024) {
            message = (try? JSONDecoder().decode(ErrorResponseDTO.self, from: data))?.error
        }
        throw BeGitAPIError.requestFailed(statusCode: response.status.code, message: message)
    }
}

struct BeGitBackendAPI: AuthAPI, RepositoryAPI, CurrentUserAPI {
    private let baseURL: URL
    private let session: URLSession
    
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
    
    // openapi.yaml の servers は相対(/)なので serverURL に実行時 baseURL を指定する。
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
    
    func exchangeCode(code: String) async throws -> AuthResponse {
        let output = try await makeClient().postAuthGithub(
            .init(body: .json(.Handler_AuthRequest(.init(code: code))))
        )
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        let payload = try ok.body.json
        
        // 必須フィールドの検証
        guard let token = payload.token, !token.isEmpty else {
            throw BeGitAPIError.invalidResponse
        }
        guard let user = payload.user, let userId = user.id, let userLogin = user.login else {
            throw BeGitAPIError.invalidResponse
        }
        
        return AuthResponse(
            accessToken: token,
            githubUser: GitHubUser(
                id: userId,
                login: userLogin,
                name: user.name,
                avatarURL: user.avatarUrl.flatMap { URL(string: $0) },
                email: nil
            )
        )
    }
    
    func listRepositories(accessToken: String) async throws -> [Repository] {
        let output = try await makeClient(accessToken: accessToken).getGroups()
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        return (try ok.body.json.groups ?? []).map { $0.toRepository() }
    }

    /// GitHub AppのInstallation範囲を含む候補リポジトリをバックエンドから取得する。
    /// 生成クライアントに依存せず、既存のGET /github/reposへInstallation IDを渡す。
    func listGitHubRepositories(accessToken: String, installationID: Int64) async throws -> [GitHubRepository] {
        guard installationID > 0 else { throw BeGitAPIError.invalidResponse }

        var components = URLComponents(
            url: baseURL.appending(path: "github/repos"),
            resolvingAgainstBaseURL: false
        )
        components?.queryItems = [
            URLQueryItem(name: "installation_id", value: String(installationID))
        ]
        guard let url = components?.url else { throw BeGitAPIError.invalidURL }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Accept")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw BeGitAPIError.invalidResponse
        }
        guard (200..<300).contains(httpResponse.statusCode) else {
            if httpResponse.statusCode == 401 {
                throw BeGitAPIError.authenticationRequired
            }
            let message = (try? JSONDecoder().decode(ErrorResponseDTO.self, from: data))?.error
            throw BeGitAPIError.requestFailed(statusCode: httpResponse.statusCode, message: message)
        }

        let payload = try JSONDecoder().decode(GitHubRepoListResponseDTO.self, from: data)
        return payload.repos.map { repo in
            GitHubRepository(
                id: repo.id,
                fullName: repo.fullName,
                description: nil,
                isPrivate: repo.private,
                ownerAvatarURL: repo.avatarURL.isEmpty ? nil : URL(string: repo.avatarURL),
                updatedAt: nil
            )
        }
    }
    
    // 作成成功は 201(.created)
    func createRepository(
        repoFullName: String,
        name: String,
        installationID: Int64?,
        readOnly: Bool,
        accessToken: String
    ) async throws -> Repository {
        let output = try await makeClient(accessToken: accessToken).postGroups(
            .init(body: .json(.Handler_CreateGroupRequest(.init(
                installationId: installationID.map { Int($0) },
                name: name,
                readOnly: readOnly, repoFullName: repoFullName
            ))))
        )
        guard case let .created(created) = output else { throw BeGitAPIError.invalidResponse }
        guard let id = try created.body.json.id else { throw BeGitAPIError.invalidResponse }
        
        return try await getRepository(id: Int64(id), accessToken: accessToken)
    }
    
    func getRepository(id: Int64, accessToken: String) async throws -> Repository {
        let output = try await makeClient(accessToken: accessToken).getGroupsId(
            .init(path: .init(id: Int(id)))
        )
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        return try ok.body.json.toRepository()
    }

    /// DELETE /groups/:id : ログイン中ユーザーのHOMEからリポジトリを削除する。
    func deleteRepository(id: Int64, accessToken: String) async throws {
        let url = baseURL.appending(path: "groups/\(id)")
        var request = URLRequest(url: url)
        request.httpMethod = "DELETE"
        request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Accept")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw BeGitAPIError.invalidResponse
        }
        guard (200..<300).contains(httpResponse.statusCode) else {
            if httpResponse.statusCode == 401 {
                throw BeGitAPIError.authenticationRequired
            }
            let message = (try? JSONDecoder().decode(ErrorResponseDTO.self, from: data))?.error
            throw BeGitAPIError.requestFailed(statusCode: httpResponse.statusCode, message: message)
        }
    }
    
    func listActivities(repository: Repository, accessToken: String) async throws -> [RepositoryActivity] {
        guard let backendID = repository.backendID else { return [] }
        let output = try await makeClient(accessToken: accessToken).getGroupsIdPosts(
            .init(path: .init(id: Int(backendID)))
        )
        #if DEBUG
        print("API response")
        #endif
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        #if DEBUG
        print(try ok.body.json)
        #endif
        return (try ok.body.json.posts ?? []).map { $0.toActivity(fallbackRepository: repository) }
    }
    
    // 通知発行成功は 201(.created)
    func sendNotification(repositoryID: Int64, accessToken: String) async throws {
        let output = try await makeClient(accessToken: accessToken).postGroupsIdNotifications(
            .init(path: .init(id: Int(repositoryID)))
        )
        guard case .created = output else { throw BeGitAPIError.invalidResponse }
    }
    
    // GET /me : Bearer トークンから現在ログイン中ユーザーを取得（GitHub 直叩きの代替）
    func getCurrentUser(accessToken: String) async throws -> GitHubUser {
        let output = try await makeClient(accessToken: accessToken).getMe()
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        return try ok.body.json.toGitHubUser()
    }
    
    func createPost(
        repositoryID: Int64,
        body: String,
        repoFullName: String,
        githubLogin: String,
        accessToken: String
    ) async throws -> Int64 {

        let request = Components.Schemas.Handler_CreatePostRequest(
            body: body,
            githubLogin: githubLogin,
            notificationId: nil,
            repoFullName: repoFullName
        )

        let output = try await makeClient(accessToken: accessToken)
            .postGroupsIdPosts(
                path: .init(id: Int(repositoryID)),
                body: .json(
                    .Handler_CreatePostRequest(request)
                )
            )

        guard case let .created(created) = output else {
            throw BeGitAPIError.invalidResponse
        }

        let post = try created.body.json

        guard let postID = post.id else {
            throw BeGitAPIError.invalidResponse
        }

        return Int64(postID)
    }
    
    func uploadPhotos(
        repositoryID: Int64,
        postID: Int64,
        mainImageData: Data,
        frontImageData: Data?,
        accessToken: String
    ) async throws {
        let mainPart = OpenAPIRuntime.MultipartPart(
            payload: Operations.PostGroupsIdPostsPostIdPhotos
                .Input
                .Body
                .MultipartFormPayload
                .MainPayload(
                    body: HTTPBody(mainImageData)
                ),
            filename: "main.jpg"
        )

        var parts: [Operations.PostGroupsIdPostsPostIdPhotos.Input.Body.MultipartFormPayload] = []
        parts.append(.main(mainPart))

        if let frontImageData {
            let frontPart = OpenAPIRuntime.MultipartPart(
                payload: Operations.PostGroupsIdPostsPostIdPhotos
                    .Input
                    .Body
                    .MultipartFormPayload
                    .FrontPayload(
                        body: HTTPBody(frontImageData)
                    ),
                filename: "front.jpg"
            )
            parts.append(.front(frontPart))
        }

        let multipartBody = OpenAPIRuntime.MultipartBody(parts)

        let output = try await makeClient(accessToken: accessToken).postGroupsIdPostsPostIdPhotos(
            .init(
                path: .init(
                    id: Int(repositoryID),
                    postId: Int(postID)
                ),
                body: .multipartForm(multipartBody)
            )
        )

        guard case .created = output else {
            throw BeGitAPIError.invalidResponse
        }
    }

    // GET /groups/{id}/posts/{postId}/draft : ② Nice Work! の下書きを取得（確定済みなら 404）
    func getDraftPost(repositoryID: Int64, postID: Int64, accessToken: String) async throws -> DraftPost {
        let output = try await makeClient(accessToken: accessToken).getGroupsIdPostsPostIdDraft(
            path: .init(id: Int(repositoryID), postId: Int(postID))
        )
        guard case let .ok(ok) = output else { throw BeGitAPIError.invalidResponse }
        return try ok.body.json.toDraftPost(fallbackID: postID)
    }

    // POST /groups/{id}/posts/{postId}/confirm : 下書きを確定してフィードに出す（べき等）
    // body が nil のときは下書きの本文を上書きしない。
    func confirmPost(repositoryID: Int64, postID: Int64, body: String?, accessToken: String) async throws {
        let output = try await makeClient(accessToken: accessToken).postGroupsIdPostsPostIdConfirm(
            path: .init(id: Int(repositoryID), postId: Int(postID)),
            body: .json(.Handler_ConfirmPostRequest(.init(body: body)))
        )
        guard case .ok = output else { throw BeGitAPIError.invalidResponse }
    }

    // PUT /me/fcm-token : FCM デバイストークンを登録/更新（Push 送信先の登録）
    func updateFCMToken(_ token: String, accessToken: String) async throws {
        let output = try await makeClient(accessToken: accessToken).putMeFcmToken(
            .init(body: .json(.Handler_UpdateFCMTokenRequest(.init(fcmToken: token))))
        )
        guard case .ok = output else { throw BeGitAPIError.invalidResponse }
    }

    // POST /auth/logout : GitHubトークン失効 + FCMトークン削除
    func logout(accessToken: String) async throws {
        let output = try await makeClient(accessToken: accessToken).postAuthLogout()
        guard case .noContent = output else { throw BeGitAPIError.invalidResponse }
    }
}

// nonisolated 指定：アプリは MainActor 既定隔離のため、これを付けないと Decodable 適合も
// MainActor 隔離になり、上の nonisolated な ErrorThrowingMiddleware から decode できない。
private nonisolated struct ErrorResponseDTO: Decodable {
    let error: String
}

private nonisolated struct GitHubRepoListResponseDTO: Decodable {
    let repos: [GitHubRepoDTO]
}

private nonisolated struct GitHubRepoDTO: Decodable {
    let id: Int
    let fullName: String
    let name: String
    let `private`: Bool
    let ownerLogin: String
    let avatarURL: String
    let canPush: Bool
    let canAdmin: Bool

    enum CodingKeys: String, CodingKey {
        case id
        case fullName = "full_name"
        case name
        case `private`
        case ownerLogin = "owner_login"
        case avatarURL = "avatar_url"
        case canPush = "can_push"
        case canAdmin = "can_admin"
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decodeIfPresent(Int.self, forKey: .id) ?? 0
        fullName = try container.decodeIfPresent(String.self, forKey: .fullName) ?? ""
        name = try container.decodeIfPresent(String.self, forKey: .name) ?? ""
        `private` = try container.decodeIfPresent(Bool.self, forKey: .private) ?? false
        ownerLogin = try container.decodeIfPresent(String.self, forKey: .ownerLogin) ?? ""
        avatarURL = try container.decodeIfPresent(String.self, forKey: .avatarURL) ?? ""
        canPush = try container.decodeIfPresent(Bool.self, forKey: .canPush) ?? false
        canAdmin = try container.decodeIfPresent(Bool.self, forKey: .canAdmin) ?? false
    }
}
