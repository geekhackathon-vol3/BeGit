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

        //  既定はDebug/ReleaseともにバックエンドのBeGit Timeを発行する。
        //  これにより、Xcodeから動かす開発版でもnotification_id付きの投稿になり、
        //  タイムラインの「投稿して表示」ロックを正しく解除できる。
        //  端末内だけで試したい場合は Scheme の環境変数
        //  BEGIT_NOTIFICATION_DELIVERY_MODE=localMock を明示的に指定する。
        return .remotePush
    }

    var usesLocalNotificationMock: Bool {
        self == .localMock
    }
}
