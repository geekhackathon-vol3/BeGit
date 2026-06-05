//  NotificationResultViewModel.swift
//  通知送信結果画面のMock状態管理

import Foundation
import Combine

@MainActor
final class NotificationResultViewModel: ObservableObject {
    let notification: RepositoryNotification                        //  通知結果情報
    @Published private(set) var activities: [RepositoryActivity]    //  Timeline表示用activity一覧
    @Published private(set) var completedCount: Int                 //  達成済みmember数

    init(notification: RepositoryNotification) {
        self.notification = notification
        let mock = RepositoryActivity.mockActivities(for: notification.repository)
        self.activities = mock
        self.completedCount = mock.count    //  モックアクティビティ数に一致
    }

    //  通知対象member総数（モックはactivity数で揃える）
    var totalCount: Int {
        activities.count
    }

    //  達成率
    var progress: Double {
        Double(completedCount) / Double(totalCount)
    }

    //  達成状況表示テキスト
    var progressText: String {
        "\(completedCount)/\(totalCount)人が達成しました"
    }
}

