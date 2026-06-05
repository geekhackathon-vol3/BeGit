//  NotificationResultViewModel.swift
//  通知送信結果画面のMock状態管理

import Foundation
import Combine

@MainActor
final class NotificationResultViewModel: ObservableObject {
    let notification: RepositoryNotification                        //  通知結果情報
    @Published private(set) var activities: [RepositoryActivity]    //  Timeline表示用activity一覧
    @Published private(set) var completedCount: Int                 //  達成済みmember数
    @Published private(set) var isLoading = false                   //  フィード取得中

    private let repositoryAPI: any RepositoryAPI

    init(
        notification: RepositoryNotification,
        repositoryAPI: any RepositoryAPI = BeGitBackendAPI()
    ) {
        self.notification = notification
        self.repositoryAPI = repositoryAPI
        let mock = RepositoryActivity.mockActivities(for: notification.repository)
        self.activities = mock
        self.completedCount = mock.count    //  モックアクティビティ数に一致
    }
    //  バックエンドのフィード（実写真付き）を取得して Timeline を差し替える
    func loadActivities(accessToken: String?) async {
        guard let accessToken, accessToken.isEmpty == false,
              notification.repository.backendID != nil else {
            return
        }

        isLoading = true
        defer { isLoading = false }

        do {
            let fetched = try await repositoryAPI.listActivities(
                repository: notification.repository,
                accessToken: accessToken
            )
            activities = fetched
        } catch {
            //  取得失敗時は初期 Mock のまま表示を維持する
        }
    }

    //  通知対象member総数
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

