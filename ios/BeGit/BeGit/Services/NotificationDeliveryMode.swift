//  NotificationDeliveryMode.swift
//  通知の配送方法を 1 か所で切り替えるための設定

import Foundation

enum NotificationDeliveryMode: String {
    case localMock
    case remotePush

    static var current: NotificationDeliveryMode {
        if let rawValue = ProcessInfo.processInfo.environment["BEGIT_NOTIFICATION_DELIVERY_MODE"],
           let mode = NotificationDeliveryMode(rawValue: rawValue) {
            return mode
        }

        if UserDefaults.standard.bool(forKey: "useRemotePushNotifications") {
            return .remotePush
        }

        //  既定：Xcode から Run する Debug ビルドはローカルモック、
        //  TestFlight / App Store 向けの Release ビルドはサーバー経由の Push。
        //  （Scheme の環境変数は Run 時しか効かないため、配布ビルドはここで決める）
        #if DEBUG
        return .localMock
        #else
        return .remotePush
        #endif
    }

    var usesLocalNotificationMock: Bool {
        self == .localMock
    }
}
