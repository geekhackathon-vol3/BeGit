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
    //  進行中のBeGit Time（GET /groups/:id の active_challenge）。nil = 進行中なし
    @Published private(set) var activeChallenge: ActiveChallenge?
    @Published private(set) var isEndingChallenge = false           //  途中終了API呼び出し中
    @Published var challengeErrorMessage: String?                   //  途中終了の失敗表示

    private let repositoryAPI: any RepositoryAPI

    init(
        repository: Repository,
        activities: [RepositoryActivity]? = nil,
        activeChallenge: ActiveChallenge? = nil,
        repositoryAPI: any RepositoryAPI = BeGitBackendAPI()
    ) {
        self.repository = repository
        self.activities = activities ?? []
        self.activeChallenge = activeChallenge ?? repository.activeChallenge
        self.repositoryAPI = repositoryAPI
    }

    //  Timelineの達成サマリーは実際のRepository memberを表示する。
    //  投稿カードはデモ用モックを維持するため、activityのauthor数とは分離する。
    var completedCount: Int {
        totalCount
    }

    //  リポジトリ総member数
    var totalCount: Int {
        max(repository.memberCount, repository.members.count)
    }

    //  達成率
    var progress: Double {
        guard totalCount > 0 else { return 0 }
        return Double(completedCount) / Double(totalCount)
    }

    //  達成状況テキスト
    var progressText: String {
        "\(completedCount)/\(totalCount)人が達成しました"
    }

    func loadActivities(accessToken: String?, currentUserID: Int64? = nil) async {
        let mock = RepositoryActivity.mockActivities(for: repository)

        guard let accessToken, repository.backendID != nil else {
            activities = mock
            return
        }

        isLoading = true
        errorMessage = nil
        defer { isLoading = false }

        do {
            var fetched = try await repositoryAPI.listActivities(repository: repository, accessToken: accessToken)
            if let repositoryID = repository.backendID {
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

    func toggleReaction(
        activityID: UUID,
        type: ActivityReactionType,
        accessToken: String?,
        currentUserID: Int64?
    ) async throws -> [ActivityReaction]? {
        guard let accessToken,
              let repositoryID = repository.backendID,
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
            // 種類変更のDELETE後にPOSTが失敗しても、表示をサーバ状態に戻す。
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

    //  進行中のBeGit Timeを取得する（画面表示・復帰・締め切り到達・終了後に呼ぶ）。
    //  取得失敗時は前回の状態を維持する（バナーの点滅を避ける）。
    func loadActiveChallenge(accessToken: String?) async {
        guard let accessToken, let backendID = repository.backendID else {
            activeChallenge = nil
            return
        }

        do {
            let synced = try await repositoryAPI.getRepository(id: backendID, accessToken: accessToken)
            activeChallenge = synced.activeChallenge
        } catch {
            //  失敗しても既存表示を維持
        }
    }

    //  発行者が進行中のBeGit Timeを途中終了する（締め切りを今にする）。成功後はバナーを消す。
    func endChallenge(accessToken: String?) async {
        guard isEndingChallenge == false,
              let accessToken,
              let backendID = repository.backendID,
              let challenge = activeChallenge else { return }

        isEndingChallenge = true
        challengeErrorMessage = nil
        defer { isEndingChallenge = false }

        do {
            try await repositoryAPI.endChallenge(
                repositoryID: backendID,
                notificationID: challenge.notificationID,
                accessToken: accessToken
            )
            activeChallenge = nil
        } catch BeGitAPIError.requestFailed(statusCode: 409, message: _) {
            //  既に終了済み／1時間経過済み → 最新状態に合わせる
            await loadActiveChallenge(accessToken: accessToken)
        } catch BeGitAPIError.requestFailed(statusCode: 403, message: _) {
            challengeErrorMessage = "発行者だけがBeGit Timeを終了できます。"
            await loadActiveChallenge(accessToken: accessToken)
        } catch {
            challengeErrorMessage = "BeGit Timeの終了に失敗しました。"
        }
    }
}
