//  AuthState.swift
//  GitHubログイン状態を管理するStateオブジェクト

import Combine
import Foundation

@MainActor
final class AuthState: ObservableObject {
    static let shared = AuthState(keychainManager: KeychainManager())   // アプリ全体で共有する認証状態

    @Published var isLoggedIn = false   // ログイン状態
    @Published var accessToken: String? // GitHubアクセストークン
    @Published var githubUser: GitHubUser?  // ログイン中のGitHubユーザー情報

    private let keychainManager: KeychainManaging   // トークン保存用Keychain
    private let savedGitHubUserKey = "savedGitHubUser"

    init(keychainManager: any KeychainManaging) {
        self.keychainManager = keychainManager
        restoreSession()
    }

    //  前回ログイン情報を復元する
    func restoreSession() {
        let restoredSavedSession = restoreSavedSession()

#if DEBUG
        if devSessionEnabled && !restoredSavedSession {
            applyDevSession()
        }
#endif
    }

    private func restoreSavedSession() -> Bool {
        do {
            accessToken = try keychainManager.readAccessToken()
            githubUser = restoreSavedGitHubUser()
            isLoggedIn = accessToken != nil
            return isLoggedIn
        } catch {
            accessToken = nil
            githubUser = nil
            isLoggedIn = false
            return false
        }
    }

    //  ログイン成功処理
    func completeLogin(response: AuthResponse) {
        accessToken = response.accessToken
        githubUser = response.githubUser
        isLoggedIn = true
        saveGitHubUser(response.githubUser)
        //  ログイン直後に FCM トークンを DB へ登録する（PUT /me/fcm-token）
        FCMTokenRegistrar.shared.registerAfterLogin()
    }

    func updateGitHubUser(_ githubUser: GitHubUser) {
        self.githubUser = githubUser
        saveGitHubUser(githubUser)
    }

    //  ログアウト処理
    func logout() {
        do {
            try keychainManager.deleteAccessToken()
        } catch {
            // Keychainの削除に失敗しても、ログアウト状態にはする
        }

        accessToken = nil
        githubUser = nil
        isLoggedIn = false
        UserDefaults.standard.removeObject(forKey: savedGitHubUserKey)
        //  FCM トークンのキャッシュをクリアして、次のユーザーログイン時に再送信されるようにする
        FCMTokenRegistrar.shared.clearCache()
    }

    private func applyDevSession() {
        accessToken = "dev_alice"
        githubUser = GitHubUser(
            id: 1,
            login: "dev_alice",
            name: "dev_alice (dev)",
            avatarURL: nil,
            email: nil
        )
        isLoggedIn = true
    }

    private var devSessionEnabled: Bool {
        let environmentValue = ProcessInfo.processInfo.environment["BEGIT_DEV_SESSION_ENABLED"]
        return environmentValue == "1" || UserDefaults.standard.bool(forKey: "devSessionEnabled")
    }

    private func restoreSavedGitHubUser() -> GitHubUser? {
        guard let data = UserDefaults.standard.data(forKey: savedGitHubUserKey),
              let savedUser = try? JSONDecoder().decode(SavedGitHubUser.self, from: data) else {
            return nil
        }

        return savedUser.githubUser
    }

    private func saveGitHubUser(_ githubUser: GitHubUser) {
        guard let data = try? JSONEncoder().encode(SavedGitHubUser(githubUser: githubUser)) else {
            return
        }

        UserDefaults.standard.set(data, forKey: savedGitHubUserKey)
    }
}

private struct SavedGitHubUser: Codable {
    let id: Int
    let login: String
    let name: String?
    let avatarURLString: String?
    let email: String?

    init(githubUser: GitHubUser) {
        id = githubUser.id
        login = githubUser.login
        name = githubUser.name
        avatarURLString = githubUser.avatarURL?.absoluteString
        email = githubUser.email
    }

    var githubUser: GitHubUser {
        GitHubUser(
            id: id,
            login: login,
            name: name,
            avatarURL: avatarURLString.flatMap(URL.init(string:)),
            email: email
        )
    }
}
