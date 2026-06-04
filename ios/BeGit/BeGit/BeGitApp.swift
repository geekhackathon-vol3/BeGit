//
//  BeGitApp.swift
//  BeGit
//  アプリの起動設定を行うエントリーポイント
//
//  Created by palm on 2026/05/24.
//

import SwiftUI
import UIKit

@main
struct BeGitApp: App {
    //  SwiftUI アプリに AppDelegate（通知・Firebase の司令塔）を後付けで接続する
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate

    init() {
        Self.configureNavigationBar()

        GitHubOAuthManager.shared.configure(
            // 認証状態管理クラス
            authState: AuthState.shared,
            // API通信クラス
            authAPI: BeGitBackendAPI(),
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

    // NavigationBarの戻る矢印をBeGitデザインに差し替える
    private static func configureNavigationBar() {
        guard let backArrow = UIImage(named: "begit_back_arrow")?.resized(to: CGSize(width: 22, height: 22)) else {
            return
        }

        let appearance = UINavigationBarAppearance()
        appearance.configureWithTransparentBackground()
        appearance.setBackIndicatorImage(backArrow, transitionMaskImage: backArrow)

        UINavigationBar.appearance().standardAppearance = appearance
        UINavigationBar.appearance().compactAppearance = appearance
        UINavigationBar.appearance().scrollEdgeAppearance = appearance
        UINavigationBar.appearance().tintColor = UIColor(AppTheme.softPink)
    }
}

private extension UIImage {
    func resized(to size: CGSize) -> UIImage {
        let renderer = UIGraphicsImageRenderer(size: size)
        return renderer.image { _ in
            draw(in: CGRect(origin: .zero, size: size))
        }.withRenderingMode(.alwaysOriginal)
    }
}
