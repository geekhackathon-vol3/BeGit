//  AuthAPI.swift
//  GitHub認証APIのインターフェース・ダミー実装

import Foundation

//  GitHub認証APIのインターフェース
protocol AuthAPI: Sendable {
    func exchangeCode(code: String) async throws -> AuthResponse
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
