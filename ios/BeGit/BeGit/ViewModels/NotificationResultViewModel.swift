//  NotificationResultViewModel.swift
//  通知送信結果画面のMock状態管理

import Foundation
import Combine

@MainActor
final class NotificationResultViewModel: ObservableObject {
    let notification: RepositoryNotification                        //  通知結果情報
    @Published private(set) var activities: [RepositoryActivity]    //  Timeline表示用activity一覧
    @Published private(set) var activeBeGitTime: ActiveBeGitTime?
    @Published private(set) var endedBeGitTime: ActiveBeGitTime?
    @Published private(set) var notificationMemberStatuses: [NotificationMemberStatus] = []
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
            }
        } catch {
            //  取得失敗時は初期 Mock のまま表示を維持する
        }

        //  フィード取得に失敗しても、BeGit Timeと参加状況の表示取得は継続する。
        await loadNotificationStatus(accessToken: accessToken)
        await loadActiveBeGitTime(accessToken: accessToken)
    }

    func loadActiveBeGitTime(accessToken: String?) async {
        guard let accessToken,
              let repositoryID = notification.repository.backendID,
              let notificationID = notification.backendID else {
            activeBeGitTime = nil
            return
        }

        do {
            let active = try await repositoryAPI.getActiveBeGitTime(
                repositoryID: repositoryID,
                accessToken: accessToken
            )
            if let active, active.notificationID == notificationID {
                activeBeGitTime = active
                endedBeGitTime = nil
            } else if let cached = ActiveBeGitTimeStore.load(repositoryID: repositoryID) {
                //  APIが未反映の環境でも、送信直後の有効なキャッシュで表示する。
                activeBeGitTime = cached
            } else {
                activeBeGitTime = nil
            }
        } catch {
            let cached = ActiveBeGitTimeStore.load(repositoryID: repositoryID)
                activeBeGitTime = cached.flatMap { cached in
                    cached.notificationID == 0 || cached.notificationID == notificationID ? cached : nil
                }
        }
    }

    func loadNotificationStatus(accessToken: String?) async {
        guard let accessToken,
              let repositoryID = notification.repository.backendID,
              let notificationID = notification.backendID else {
            notificationMemberStatuses = []
            return
        }

        do {
            notificationMemberStatuses = try await repositoryAPI.getNotificationStatus(
                repositoryID: repositoryID,
                notificationID: notificationID,
                accessToken: accessToken
            )
        } catch {
            notificationMemberStatuses = []
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

    func stopActiveBeGitTime(accessToken: String?) async throws {
        guard let accessToken,
              let repositoryID = notification.repository.backendID else {
            throw BeGitAPIError.invalidResponse
        }

        guard let active = activeBeGitTime else {
            throw BeGitAPIError.invalidResponse
        }

        // Debugのローカル通知モードにはバックエンド通知IDがないため、
        // サーバーAPIではなく端末内のBeGit Timeを停止する。
        if NotificationDeliveryMode.current.usesLocalNotificationMock {
            endedBeGitTime = active
            activeBeGitTime = nil
            ActiveBeGitTimeStore.remove(repositoryID: repositoryID)
            notificationMemberStatuses = []
            return
        }

        let serverActive: ActiveBeGitTime
        if active.notificationID > 0 {
            serverActive = active
        } else if let fetched = try await repositoryAPI.getActiveBeGitTime(
            repositoryID: repositoryID,
            accessToken: accessToken
        ) {
            serverActive = fetched
            activeBeGitTime = fetched
        } else {
            throw BeGitAPIError.invalidResponse
        }

        try await repositoryAPI.stopNotification(
            repositoryID: repositoryID,
            notificationID: serverActive.notificationID,
            accessToken: accessToken
        )

        endedBeGitTime = serverActive
        activeBeGitTime = nil
        ActiveBeGitTimeStore.remove(repositoryID: repositoryID)
        await loadNotificationStatus(accessToken: accessToken)
    }

    func deleteActivity(_ activity: RepositoryActivity, accessToken: String?) async throws {
        guard let accessToken,
              let repositoryID = notification.repository.backendID,
              let postID = activity.backendPostID else {
            throw BeGitAPIError.invalidResponse
        }

        try await repositoryAPI.deletePost(
            repositoryID: repositoryID,
            postID: postID,
            accessToken: accessToken
        )
        removeDeletedActivity(activity)
        await loadNotificationStatus(accessToken: accessToken)
    }

    private func removeDeletedActivity(_ activity: RepositoryActivity) {
        if let backendPostID = activity.backendPostID {
            activities.removeAll {
                $0.id == activity.id || $0.backendPostID == backendPostID
            }
        } else {
            activities.removeAll { $0.id == activity.id }
        }
    }
}
