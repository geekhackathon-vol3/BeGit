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
        justPostedActivity: RepositoryActivity? = nil,
        repositoryAPI: any RepositoryAPI = BeGitBackendAPI()
    ) {
        self.notification = notification
        self.repositoryAPI = repositoryAPI
        let mock = RepositoryActivity.mockActivities(for: notification.repository)
        //  デモ投稿がある場合は先頭に追加して即時表示
        let initial = justPostedActivity.map { [$0] + mock } ?? mock
        self.activities = initial
        //  デモ時は達成人数を 3/4 に固定
        self.completedCount = justPostedActivity != nil ? 3 : mock.count
    }
    //  バックエンドのフィード（実写真付き）を取得して Timeline を差し替える
    func loadActivities(accessToken: String?, currentUserID: Int64? = nil) async {
        guard let accessToken, accessToken.isEmpty == false,
              notification.repository.backendID != nil else {
            return
        }

        isLoading = true
        defer { isLoading = false }

        do {
            var fetched = try await repositoryAPI.listActivities(
                repository: notification.repository,
                accessToken: accessToken
            )
            if let repositoryID = notification.repository.backendID {
                for index in fetched.indices {
                    guard let postID = fetched[index].backendPostID else { continue }
                    if let reactions = try? await repositoryAPI.listReactions(
                        repositoryID: repositoryID,
                        postID: postID,
                        currentUserID: currentUserID,
                        accessToken: accessToken
                    ) {
                        fetched[index].reactions = reactions
                    }
                }
            }
            if !fetched.isEmpty {
                //  実投稿（新しい順）をモックの上に積み重ねる
                let mock = RepositoryActivity.mockActivities(for: notification.repository)
                activities = fetched + mock
                completedCount = Set(activities.map(\.author.login)).count
            }
        } catch {
            //  取得失敗時は初期 Mock のまま表示を維持する
        }
    }

    func toggleReaction(
        activityID: UUID,
        type: ActivityReactionType,
        accessToken: String?,
        currentUserID: Int64?
    ) async throws -> [ActivityReaction]? {
        guard let accessToken,
              let repositoryID = notification.repository.backendID,
              let index = activities.firstIndex(where: { $0.id == activityID }),
              let postID = activities[index].backendPostID else {
            return nil
        }

        let myReactionTypes = activities[index].reactions
            .filter(\.reactedByMe)
            .map(\.type)

        do {
            let updated: [ActivityReaction]
            if myReactionTypes.contains(type) {
                updated = try await repositoryAPI.deleteReaction(
                    type,
                    repositoryID: repositoryID,
                    postID: postID,
                    currentUserID: currentUserID,
                    accessToken: accessToken
                )
            } else {
                for previousType in myReactionTypes {
                    _ = try await repositoryAPI.deleteReaction(
                        previousType,
                        repositoryID: repositoryID,
                        postID: postID,
                        currentUserID: currentUserID,
                        accessToken: accessToken
                    )
                }
                updated = try await repositoryAPI.addReaction(
                    type,
                    repositoryID: repositoryID,
                    postID: postID,
                    currentUserID: currentUserID,
                    accessToken: accessToken
                )
            }

            updateReactions(updated, for: activityID)
            return updated
        } catch {
            if let current = try? await repositoryAPI.listReactions(
                repositoryID: repositoryID,
                postID: postID,
                currentUserID: currentUserID,
                accessToken: accessToken
            ) {
                updateReactions(current, for: activityID)
            }
            throw error
        }
    }

    private func updateReactions(_ reactions: [ActivityReaction], for activityID: UUID) {
        if let index = activities.firstIndex(where: { $0.id == activityID }) {
            activities[index].reactions = reactions
        }
    }

    //  通知対象member総数（デモ投稿分は除いてモック4件固定）
    var totalCount: Int {
        mockCount
    }

    private let mockCount = 4

    //  達成率
    var progress: Double {
        Double(completedCount) / Double(totalCount)
    }

    //  達成状況表示テキスト
    var progressText: String {
        "\(completedCount)/\(totalCount)人が達成しました"
    }
}
