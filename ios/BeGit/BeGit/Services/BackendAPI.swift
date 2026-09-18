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
            if statusCode == 403 {
                return "このリポジトリへのアクセス権がありません。所有者にGitHub Appをインストールして、対象リポジトリを選択してもらってください。"
            }
            if statusCode == 502 {
                return "GitHub App経由のリポジトリ登録に失敗しました。Appのインストール先とリポジトリ権限を確認してください。"
            }
            return message ?? "APIリクエストに失敗しました。status=\(statusCode)"
        }
    }
}

struct ActiveBeGitTime: Sendable {
    let notificationID: Int64
    let sentBy: Int64
    let sentAt: Date
    let expiresAt: Date
}

struct NotificationMemberStatus: Sendable, Identifiable {
    let id: Int64
    let login: String
    let status: String
}

// 新しいactive通知APIが未反映の環境でも、送信直後の表示を維持するための互換キャッシュ。
enum ActiveBeGitTimeStore {
    private static let keyPrefix = "begit.active-time"

    static func save(
        repositoryID: Int64,
        notificationID: Int64 = 0,
        sentBy: Int64 = 0,
        sentAt: Date = Date(),
        expiresAt: Date
    ) {
        UserDefaults.standard.set(
            [
                Double(notificationID),
                Double(sentBy),
                sentAt.timeIntervalSince1970,
                expiresAt.timeIntervalSince1970
            ],
            forKey: key(for: repositoryID)
        )
    }

    static func load(repositoryID: Int64, now: Date = Date()) -> ActiveBeGitTime? {
        guard let values = UserDefaults.standard.array(forKey: key(for: repositoryID)) as? [Double],
              values.count == 2 || values.count == 3 || values.count == 4 else {
            return nil
        }

        let notificationID: Int64
        let sentBy: Int64
        let sentAt: Date
        let expiresAt: Date
        if values.count == 4 {
            notificationID = Int64(values[0])
            sentBy = Int64(values[1])
            sentAt = Date(timeIntervalSince1970: values[2])
            expiresAt = Date(timeIntervalSince1970: values[3])
        } else if values.count == 3 {
            notificationID = 0
            sentBy = Int64(values[0])
            sentAt = Date(timeIntervalSince1970: values[1])
            expiresAt = Date(timeIntervalSince1970: values[2])
        } else {
            notificationID = 0
            sentBy = 0
            sentAt = Date(timeIntervalSince1970: values[0])
            expiresAt = Date(timeIntervalSince1970: values[1])
        }
        guard expiresAt > now else {
            remove(repositoryID: repositoryID)
            return nil
        }

        return ActiveBeGitTime(
            notificationID: notificationID,
            sentBy: sentBy,
            sentAt: sentAt,
            expiresAt: expiresAt
        )
    }

    static func remove(repositoryID: Int64) {
        UserDefaults.standard.removeObject(forKey: key(for: repositoryID))
    }

    private static func key(for repositoryID: Int64) -> String {
        "\(keyPrefix).\(repositoryID)"
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
    func listGitHubRepositories(accessToken: String, installationID: Int64) async throws -> [GitHubRepository]
    func createRepository(
        repoFullName: String,
        name: String,
        installationID: Int64?,
        readOnly: Bool,
        accessToken: String
    ) async throws -> Repository
    func getRepository(id: Int64, accessToken: String) async throws -> Repository
    func deleteRepository(id: Int64, accessToken: String) async throws
    func listActivities(repository: Repository, accessToken: String) async throws -> [RepositoryActivity]
    func sendNotification(repositoryID: Int64, accessToken: String) async throws -> Int64
    func stopNotification(repositoryID: Int64, notificationID: Int64, accessToken: String) async throws
    func getActiveBeGitTime(repositoryID: Int64, accessToken: String) async throws -> ActiveBeGitTime?
    func getNotificationStatus(repositoryID: Int64, notificationID: Int64, accessToken: String) async throws -> [NotificationMemberStatus]
    func deletePost(repositoryID: Int64, postID: Int64, accessToken: String) async throws
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
