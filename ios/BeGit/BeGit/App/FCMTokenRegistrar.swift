//  FCMTokenRegistrar.swift
//  FCM トークンの「DB 保存フロー」を一手に引き受けるコーディネータ。
//  トークン更新（AppDelegate）とログイン完了（AuthState）の2つの起点から呼ばれ、
//  ログイン済みのときだけ PUT /me/fcm-token を叩く。二重送信は抑制する。
//
//  ⚠️ FirebaseMessaging（SPM）に依存する。Xcode でパッケージ追加後に有効。

import FirebaseMessaging
import Foundation

@MainActor
final class FCMTokenRegistrar {
    static let shared = FCMTokenRegistrar()

    private let api: any CurrentUserAPI
    //  ユーザーごとに送信済みトークンを記録（キー: userID、値: FCM token）
    private var lastSentTokenByUser: [Int: String] = [:]

    init(api: any CurrentUserAPI = BeGitBackendAPI()) {
        self.api = api
    }

    //  FCM トークンが更新された（AppDelegate の MessagingDelegate から）
    func handleTokenRefresh(_ token: String) {
        send(token: token)
    }

    //  ログイン直後に、現在の FCM トークンを取得して送る（AuthState.completeLogin から）
    func registerAfterLogin() {
        Messaging.messaging().token { [weak self] token, _ in
            guard let token else { return }
            Task { @MainActor in
                self?.send(token: token)
            }
        }
    }

    //  ログアウト時にキャッシュをクリアする（AuthState.logout から呼ぶ）
    func clearCache() {
        lastSentTokenByUser.removeAll()
    }

    //  ログイン済みなら DB へ保存。同一ユーザーに対して直近送信済みと同一トークンなら skip。
    private func send(token: String) {
        guard let accessToken = AuthState.shared.accessToken,
              accessToken.isEmpty == false,
              let userId = AuthState.shared.githubUser?.id else {
            return
        }

        //  このユーザーに対して既に同じトークンを送信済みなら skip
        if lastSentTokenByUser[userId] == token {
            return
        }

        Task {
            do {
                try await api.updateFCMToken(token, accessToken: accessToken)
                lastSentTokenByUser[userId] = token
            } catch {
                //  失敗しても次回のトークン更新／再ログインで再送される
            }
        }
    }
}
