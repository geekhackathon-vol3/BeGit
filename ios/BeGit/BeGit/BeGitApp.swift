//
//  BeGitApp.swift
//  BeGit
//  アプリの起動設定を行うエントリーポイント
//
//  Created by palm on 2026/05/24.
//

import SwiftUI

@main
struct BeGitApp: App {
    init() {
        GitHubOAuthManager.shared.configure(
            // 認証状態管理クラス
            authState: AuthState.shared,
            // API通信クラス
            authAPI: MockAuthAPI(),
            // Keychain保存クラス
            keychainManager: KeychainManager()
        )
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(AuthState.shared)    // ログイン状態をアプリ全体で共有
        }
    }
}
