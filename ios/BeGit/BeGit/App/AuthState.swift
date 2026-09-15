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
    /// GitHub Appのインストール単位。未連携のユーザーはnilのままOAuth方式を使う。
    @Published private(set) var githubAppInstallationID: Int64?

    private let keychainManager: KeychainManaging   // トークン保存用Keychain
    private let savedGitHubUserKey = "savedGitHubUser"
    private let savedGitHubAppInstallationIDKey = "savedGitHubAppInstallationID"

    init(keychainManager: any KeychainManaging) {
        self.keychainManager = keychainManager
        restoreSession()
    }

    //  前回ログイン情報を復元する
    func restoreSession() {
        // GitHub AppのInstallationはOAuthセッションとは独立しているため、
        // Keychainのトークン有無にかかわらず先に復元する。
        githubAppInstallationID = restoreSavedGitHubAppInstallationID()
        restoreSavedSession()
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
        githubAppInstallationID = restoreSavedGitHubAppInstallationID()
        isLoggedIn = true
        saveGitHubUser(response.githubUser)
        //  ログイン直後に FCM トークンを DB へ登録する（PUT /me/fcm-token）
        FCMTokenRegistrar.shared.registerAfterLogin()
    }

    func updateGitHubUser(_ githubUser: GitHubUser) {
        self.githubUser = githubUser
        saveGitHubUser(githubUser)
    }

    /// GitHub AppのSetup URL完了後に得たinstallation_idを保存する。
    /// 0以下を受け取った場合は連携を解除する。
    func setGitHubAppInstallationID(_ installationID: Int64?) {
        guard let installationID, installationID > 0 else {
            githubAppInstallationID = nil
            UserDefaults.standard.removeObject(forKey: savedGitHubAppInstallationIDKey)
            return
        }

        githubAppInstallationID = installationID
        UserDefaults.standard.set(installationID, forKey: savedGitHubAppInstallationIDKey)
    }

    /// GitHub App Setup URLから戻ったカスタムURLを処理する。
    @discardableResult
    func handleGitHubAppInstallationURL(_ url: URL) -> Bool {
        guard url.scheme == "begit", url.host == "github-app-setup" else {
            return false
        }

        let components = URLComponents(url: url, resolvingAgainstBaseURL: false)
        guard let rawInstallationID = components?.queryItems?.first(where: { $0.name == "installation_id" })?.value,
              let installationID = Int64(rawInstallationID),
              installationID > 0 else {
            return false
        }

        setGitHubAppInstallationID(installationID)
        return true
    }

    //  ログアウト処理
    func logout() {
        // バックエンドでGitHub OAuthトークンを失効させる（次回ログイン時にフル認証を求めるため）
        // ローカル状態のクリアはネットワーク結果を待たず即時実行する
        if let token = accessToken {
            Task {
                try? await BeGitBackendAPI().logout(accessToken: token)
            }
        }

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

    private func restoreSavedGitHubAppInstallationID() -> Int64? {
        guard let value = UserDefaults.standard.object(forKey: savedGitHubAppInstallationIDKey) as? NSNumber else {
            return nil
        }

        let installationID = value.int64Value
        return installationID > 0 ? installationID : nil
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
