//  NotificationResultViewModel.swift
//  通知送信結果画面のMock状態管理

import Foundation
import Combine

@MainActor
final class NotificationResultViewModel: ObservableObject {
    let notification: RepositoryNotification                        //  通知結果情報
    @Published private(set) var activities: [RepositoryActivity]    //  Timeline表示用activity一覧
    @Published private(set) var isLoading = false                   //  フィード取得中

    private let repositoryAPI: any RepositoryAPI

    init(
        notification: RepositoryNotification,
        justPostedActivity: RepositoryActivity? = nil,
        repositoryAPI: any RepositoryAPI = BeGitBackendAPI()
    ) {
        self.notification = notification
        self.repositoryAPI = repositoryAPI
        let mock = RepositoryActivity.mockActivities(for: notification.repository)
        //  デモ投稿がある場合は先頭に追加して即時表示
        let initial = justPostedActivity.map { [$0] + mock } ?? mock
        self.activities = initial
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
            if !fetched.isEmpty {
                //  実投稿（新しい順）をモックの上に積み重ねる
                let mock = RepositoryActivity.mockActivities(for: notification.repository)
                activities = fetched + mock
            }
        } catch {
            //  取得失敗時は初期 Mock のまま表示を維持する
        }
    }

    //  Resultには実際に通知対象として選択されたmemberを表示する。
    //  avatar URLが欠けた旧データはGitHubのユーザー画像URLで補完する。
    var members: [RepositoryMember] {
        let selectedMembers = notification.selectedMembers.isEmpty
            ? notification.repository.members
            : notification.selectedMembers

        return selectedMembers.map { member in
            guard member.avatarURL == nil, member.login.isEmpty == false else {
                return member
            }

            return RepositoryMember(
                id: member.id,
                backendUserID: member.backendUserID,
                login: member.login,
                avatarURL: URL(string: "https://github.com/\(member.login).png")
            )
        }
    }

    //  投稿モックのauthor数ではなく、実際の通知対象member数を使う。
    var totalCount: Int {
        members.count
    }

    var completedCount: Int {
        totalCount
    }

    //  達成率
    var progress: Double {
        guard totalCount > 0 else { return 0 }
        return Double(completedCount) / Double(totalCount)
    }

    //  達成状況表示テキスト
    var progressText: String {
        "\(completedCount)/\(totalCount)人が達成しました"
    }
}
