//  BackendAPI.swift
//  バックエンド API の契約（プロトコル）。実装は BeGitBackendAPI / MockAuthAPI。

import Foundation
import HTTPTypes
import OpenAPIRuntime

enum BeGitAPIError: LocalizedError {
    case authenticationRequired
    case invalidURL
    case invalidResponse
    case requestFailed(statusCode: Int, message: String?)

    var errorDescription: String? {
        switch self {
        case .authenticationRequired:
            return "ログイン状態を確認できません。再ログインしてください。"
        case .invalidURL:
            return "API URLが不正です。"
        case .invalidResponse:
            return "APIレスポンスを読み取れませんでした。"
        case let .requestFailed(statusCode, message):
            if statusCode == 401 {
                return "ログイン状態を確認できません。再ログインしてください。"
            }
            return message ?? "APIリクエストに失敗しました。status=\(statusCode)"
        }
    }
}

// OpenAPI Runtime は Middleware から投げられたエラーを ClientError などで
// ラップすることがある。画面側がラッパーの実装を意識せず、APIエラーを判定できる
// ように、既知の BeGitAPIError を再帰的に取り出す。
func beGitAPIError(from error: Error) -> BeGitAPIError? {
    findBeGitAPIError(in: error, depth: 0)
}

private func findBeGitAPIError(in value: Any, depth: Int) -> BeGitAPIError? {
    guard depth < 12 else { return nil }

    if let apiError = value as? BeGitAPIError {
        return apiError
    }

    // OpenAPI RuntimeのClientErrorは、Middlewareが投げたエラーを
    // underlyingErrorに保持する一方、HTTPレスポンス本体も公開している。
    // 401はここで直接判定して、ラップ構造に依存せず認証切れへ変換する。
    if let clientError = value as? ClientError,
       clientError.response?.status.code == 401 {
        return .authenticationRequired
    }

    if let error = value as? Error {
        let nsError = error as NSError
        if let underlyingError = nsError.userInfo[NSUnderlyingErrorKey] as? Error,
           let apiError = findBeGitAPIError(in: underlyingError, depth: depth + 1) {
            return apiError
        }
    }

    // RuntimeError.middlewareFailed のような関連値はタプルで保持されるため、
    // Errorとして直接キャストできないタプル／Optionalの中も再帰的に調べる。
    for child in Mirror(reflecting: value).children {
        if let apiError = findBeGitAPIError(in: child.value, depth: depth + 1) {
            return apiError
        }
    }

    return nil
}

protocol AuthAPI: Sendable {
    func exchangeCode(code: String) async throws -> AuthResponse
}

// ログイン中ユーザー情報を取得する API インターフェース（バックエンド GET /me）
protocol CurrentUserAPI: Sendable {
    func getCurrentUser(accessToken: String) async throws -> GitHubUser
    // FCM デバイストークンを登録/更新する（バックエンド PUT /me/fcm-token）
    func updateFCMToken(_ token: String, accessToken: String) async throws
    // GitHub OAuth トークンを失効させ FCM トークンを削除する（バックエンド POST /auth/logout）
    func logout(accessToken: String) async throws
}

protocol RepositoryAPI: Sendable {
    func listRepositories(accessToken: String) async throws -> [Repository]
    func createRepository(repoFullName: String, name: String, accessToken: String) async throws -> Repository
    func getRepository(id: Int64, accessToken: String) async throws -> Repository
    func listActivities(repository: Repository, accessToken: String) async throws -> [RepositoryActivity]
    func sendNotification(repositoryID: Int64, accessToken: String) async throws
    func uploadPhotos(
        repositoryID: Int64,
        postID: Int64,
        mainImageData: Data,
        frontImageData: Data?,
        accessToken: String
    ) async throws
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
                name: "The Octocat",
                avatarURL: URL(string: "https://github.com/octocat.png"),
                email: "octocat@github.com"
            )
        )
    }
}
