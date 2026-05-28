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
        self.activities = RepositoryActivity.mockActivities(for: notification.repository)   //  Mock activity生成
        self.completedCount = max(1, min(notification.selectedMembers.count, 3))            //  Mock達成人数
    }

    //  通知対象member総数
    var totalCount: Int {
        max(notification.selectedMembers.count, 1)
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

