//  AppDelegate.swift
//  アプリの「司令塔」。Firebase 初期化・通知許可・APNs/FCM トークン受領・通知タップの
//  ルーティングを担当する。SwiftUI の BeGitApp に @UIApplicationDelegateAdaptor で接続する。
//
//  ⚠️ このファイルは FirebaseCore / FirebaseMessaging（SPM: firebase-ios-sdk）に依存する。
//     Xcode でパッケージを追加するまではビルドが通らない（plan の手順1を参照）。

import FirebaseCore
import FirebaseMessaging
import UIKit
import UserNotifications

final class AppDelegate: NSObject, UIApplicationDelegate {
    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
    ) -> Bool {
        //  Firebase 初期化（GoogleService-Info.plist を読む）
        FirebaseApp.configure()

        //  FCM トークン受領のデリゲート
        Messaging.messaging().delegate = self

        //  通知の表示・タップを受け取るデリゲート
        UNUserNotificationCenter.current().delegate = self

        //  通知許可をリクエスト → 許可されたら APNs へ登録
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .badge, .sound]) { granted, error in
            if let error {
                print("Push notification authorization failed: \(error.localizedDescription)")
            }

            guard granted else {
                print("Push notification authorization was not granted.")
                return
            }

            DispatchQueue.main.async {
                application.registerForRemoteNotifications()
            }
        }

        //  アプリ未起動状態から通知タップで起動した場合の初期 route を拾う
        if let userInfo = launchOptions?[.remoteNotification] as? [AnyHashable: Any] {
            route(from: userInfo)
        }

        return true
    }

    //  APNs デバイストークンを受領 → FCM に橋渡し（FCM トークン生成のトリガ）
    func application(
        _ application: UIApplication,
        didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
    ) {
        Messaging.messaging().apnsToken = deviceToken
        let token = deviceToken.map { String(format: "%02.2hhx", $0) }.joined()
        print("APNs device token: \(token)")
    }

    func application(
        _ application: UIApplication,
        didFailToRegisterForRemoteNotificationsWithError error: Error
    ) {
        // 実機以外・APNs 未設定では失敗する。受信は出来ないが起動は継続する。
        print("APNs registration failed: \(error.localizedDescription)")
    }

    //  userInfo を parse して、遷移先を共有 Router に積む。未知 type は無視。
    private func route(from userInfo: [AnyHashable: Any]) {
        if localNotification(from: userInfo) != nil {
            NotificationRouter.shared.requestRoute(.camera)
            return
        }

        guard let payload = NotificationPayload(userInfo: userInfo),
              let route = payload.route else {
            return
        }
        Task { @MainActor in
            NotificationRouter.shared.requestRoute(route)
        }
    }

    private func localNotification(from userInfo: [AnyHashable: Any]) -> RepositoryNotification? {
        guard let type = userInfo["local_notification"] as? String,
              type == "repository_send",
              let repositoryName = userInfo["repository_name"] as? String else {
            return nil
        }

        let comment = (userInfo["comment"] as? String) ?? ""
        let selectedMemberLogins = userInfo["selected_member_logins"] as? [String] ?? []
        let selectedMembers = selectedMemberLogins.map { RepositoryMember(login: $0) }
        let repository = Repository(
            name: repositoryName,
            memberCount: max(selectedMembers.count, (userInfo["selected_member_count"] as? Int) ?? selectedMembers.count),
            members: selectedMembers
        )

        return RepositoryNotification(
            repository: repository,
            selectedMembers: selectedMembers,
            comment: comment
        )
    }
}

// MARK: - FCM トークン

extension AppDelegate: MessagingDelegate {
    //  FCM トークンが採番/更新されたら DB（PUT /me/fcm-token）へ保存する。
    //  ログイン前は token を保持できないので、ログイン済みのときだけ送る。
    //  （ログイン直後の送信は AuthState 側の completeLogin 経路がカバーする）
    func messaging(_ messaging: Messaging, didReceiveRegistrationToken fcmToken: String?) {
        guard let fcmToken else { return }
        print("FCM registration token: \(fcmToken)")

        Task { @MainActor in
            FCMTokenRegistrar.shared.handleTokenRefresh(fcmToken)
        }
    }
}

// MARK: - 通知の表示とタップ

extension AppDelegate: UNUserNotificationCenterDelegate {
    //  フォアグラウンド受信時もバナー表示する
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([.banner, .sound, .badge])
    }

    //  通知タップ時に type 別の画面へルーティングする（#55 の本体）
    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        route(from: response.notification.request.content.userInfo)
        completionHandler()
    }
}
