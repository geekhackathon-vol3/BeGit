//  BackendAPI.swift
//  バックエンド API の契約（プロトコル）。実装は BeGitBackendAPI / MockAuthAPI。

import Foundation

enum BeGitAPIError: LocalizedError {
    case invalidURL
    case invalidResponse
    case requestFailed(statusCode: Int, message: String?)

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

protocol AuthAPI: Sendable {
    func exchangeCode(code: String) async throws -> AuthResponse
}

protocol RepositoryAPI: Sendable {
    func listRepositories(accessToken: String) async throws -> [Repository]
    func createRepository(repoFullName: String, name: String, accessToken: String) async throws -> Repository
    func getRepository(id: Int64, accessToken: String) async throws -> Repository
    func listActivities(repository: Repository, accessToken: String) async throws -> [RepositoryActivity]
    func sendNotification(repositoryID: Int64, accessToken: String) async throws
}

//  開発・テスト用。ネットワークを使わず固定値を返す。
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
