//  GitHubUserAPI.swift
//  GitHub REST API（BeGit バックエンドではない）でログイン中ユーザー情報を取得する。
//  OpenAPI 生成 Client の対象外。GitHubRepositoryAPI と同じく GitHub を直接叩く。

import Foundation

// GitHub のログイン中ユーザー情報を取得する API インターフェース
protocol GitHubUserAPI: Sendable {
    func getAuthenticatedUser(accessToken: String) async throws -> GitHubUser
}

// GitHub REST API を使うログイン中ユーザー取得クライアント
struct GitHubUserClient: GitHubUserAPI {
    private let apiBaseURL = URL(string: "https://api.github.com")!
    private let session: URLSession
    private let decoder: JSONDecoder

    init(session: URLSession = .shared) {
        self.session = session
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        self.decoder = decoder
    }

    func getAuthenticatedUser(accessToken: String) async throws -> GitHubUser {
        var request = URLRequest(url: apiBaseURL.appending(path: "user"))
        request.httpMethod = "GET"
        request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/vnd.github+json", forHTTPHeaderField: "Accept")
        request.setValue("2022-11-28", forHTTPHeaderField: "X-GitHub-Api-Version")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw BeGitAPIError.invalidResponse
        }

        guard (200..<300).contains(httpResponse.statusCode) else {
            throw BeGitAPIError.requestFailed(statusCode: httpResponse.statusCode, message: nil)
        }

        return try decoder.decode(AuthenticatedGitHubUserResponse.self, from: data).githubUser
    }
}

private struct AuthenticatedGitHubUserResponse: Decodable {
    let id: Int
    let login: String
    let avatarUrl: String?
    let email: String?

    var githubUser: GitHubUser {
        GitHubUser(
            id: id,
            login: login,
            avatarURL: avatarUrl.flatMap(URL.init(string:)),
            email: email
        )
    }
}
