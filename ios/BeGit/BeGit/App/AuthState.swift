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

    init(keychainManager: any KeychainManaging) {
        self.keychainManager = keychainManager
        restoreSession()
    }

    //  前回ログイン情報を復元する
    func restoreSession() {
        do {
            accessToken = try keychainManager.readAccessToken()
            isLoggedIn = accessToken != nil
        } catch {
            accessToken = nil
            isLoggedIn = false
        }
    }

    //  ログイン成功処理
    func completeLogin(response: AuthResponse) {
        accessToken = response.accessToken
        githubUser = response.githubUser
        isLoggedIn = true
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
    }
}
