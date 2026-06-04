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

        return .localMock
    }

    var usesLocalNotificationMock: Bool {
        self == .localMock
    }
}
