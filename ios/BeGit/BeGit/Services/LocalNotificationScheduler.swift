//  LocalNotificationScheduler.swift
//  端末内で即時表示するローカル通知を送る

import Foundation
import UserNotifications

final class LocalNotificationScheduler {
    static let shared = LocalNotificationScheduler()

    private init() {}

    func scheduleNotification(for notification: RepositoryNotification) {
        let content = UNMutableNotificationContent()
        content.title = notification.repository.name
        content.body = makeBody(for: notification)
        content.sound = .default
        var userInfo: [String: Any] = [
            "local_notification": "repository_send",
            "repository_name": notification.repository.name,
            "selected_member_count": notification.selectedMembers.count,
            "selected_member_logins": notification.selectedMembers.map(\.login),
            "comment": notification.comment
        ]
        if let backendID = notification.repository.backendID {
            userInfo["backend_id"] = backendID
        }
        content.userInfo = userInfo

        let request = UNNotificationRequest(
            identifier: notification.id.uuidString,
            content: content,
            trigger: UNTimeIntervalNotificationTrigger(timeInterval: 1, repeats: false)
        )

        UNUserNotificationCenter.current().add(request) { error in
            if let error {
                print("Local notification scheduling failed: \(error.localizedDescription)")
            }
        }
    }

    private func makeBody(for notification: RepositoryNotification) -> String {
        let trimmedComment = notification.comment.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmedComment.isEmpty == false {
            return trimmedComment
        }

        return "\(notification.selectedMembers.count)人に通知しました"
    }
}
