//  RepositoryDashboardViewModel.swift
//  Repository Dashboard画面の状態管理

import Foundation
import Combine

@MainActor
final class RepositoryDashboardViewModel: ObservableObject {
    let repository: Repository                                      //  表示対象Repository
    @Published private(set) var activities: [RepositoryActivity]    //  Timeline表示用activity一覧
    @Published private(set) var activeBeGitTime: ActiveBeGitTime?
    @Published private(set) var endedBeGitTime: ActiveBeGitTime?
    @Published private(set) var notificationMemberStatuses: [NotificationMemberStatus] = []
    @Published private(set) var isLoading = false                   //  Timeline取得中
    @Published var errorMessage: String?                            //  APIエラー表示

    private let repositoryAPI: any RepositoryAPI

    init(
        repository: Repository,
        activities: [RepositoryActivity]? = nil,
        repositoryAPI: any RepositoryAPI = BeGitBackendAPI()
    ) {
        self.repository = repository
        self.activities = activities ?? []
        self.repositoryAPI = repositoryAPI
    }

    func loadActivities(accessToken: String?) async {
        let mock = RepositoryActivity.mockActivities(for: repository)

        guard let accessToken, repository.backendID != nil else {
            activities = mock
            return
        }

        isLoading = true
        errorMessage = nil
        defer { isLoading = false }

        do {
            let fetched = try await repositoryAPI.listActivities(repository: repository, accessToken: accessToken)
            //  実投稿（新しい順）をモックの上に積み重ねる
            activities = fetched + mock
        } catch {
            activities = mock
        }

        //  フィード取得に失敗しても、BeGit Timeの表示取得は継続する。
        await refreshActiveChallenge(accessToken: accessToken)
    }

    func refreshActiveChallenge(accessToken: String?) async {
        guard let accessToken, let repositoryID = repository.backendID else {
            activeBeGitTime = nil
            notificationMemberStatuses = []
            return
        }

        do {
            let active = try await repositoryAPI.getActiveBeGitTime(
                repositoryID: repositoryID,
                accessToken: accessToken
            )
            if let active {
                activeBeGitTime = active
                endedBeGitTime = nil
                if let statuses = try? await repositoryAPI.getNotificationStatus(
                    repositoryID: repositoryID,
                    notificationID: active.notificationID,
                    accessToken: accessToken
                ) {
                    notificationMemberStatuses = statuses
                }
            } else if let cached = ActiveBeGitTimeStore.load(repositoryID: repositoryID) {
                //  APIが未反映の環境でも、送信直後の有効なキャッシュで表示する。
                activeBeGitTime = cached
            } else {
                activeBeGitTime = nil
                ActiveBeGitTimeStore.remove(repositoryID: repositoryID)
                if endedBeGitTime == nil {
                    notificationMemberStatuses = []
                }
            }
        } catch {
            activeBeGitTime = ActiveBeGitTimeStore.load(repositoryID: repositoryID)
            if activeBeGitTime == nil && endedBeGitTime == nil {
                notificationMemberStatuses = []
            }
        }
    }

    func stopActiveBeGitTime(accessToken: String?) async throws {
        guard let repositoryID = repository.backendID,
              let active = activeBeGitTime else {
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

        guard let accessToken else {
            throw BeGitAPIError.invalidResponse
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
        if let statuses = try? await repositoryAPI.getNotificationStatus(
            repositoryID: repositoryID,
            notificationID: serverActive.notificationID,
            accessToken: accessToken
        ) {
            notificationMemberStatuses = statuses
        }
    }

    func deleteActivity(_ activity: RepositoryActivity, accessToken: String?) async throws {
        guard let accessToken,
              let repositoryID = repository.backendID,
              let postID = activity.backendPostID else {
            throw BeGitAPIError.invalidResponse
        }

        try await repositoryAPI.deletePost(
            repositoryID: repositoryID,
            postID: postID,
            accessToken: accessToken
        )
        removeDeletedActivity(activity)
        await refreshActiveChallenge(accessToken: accessToken)
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
