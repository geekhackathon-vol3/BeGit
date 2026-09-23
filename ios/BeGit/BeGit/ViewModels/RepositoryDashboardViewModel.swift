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
        guard let accessToken, repository.backendID != nil else {
            activities = []
            return
        }

        isLoading = true
        errorMessage = nil
        defer { isLoading = false }

        do {
            var fetched = try await repositoryAPI.listActivities(repository: repository, accessToken: accessToken)
            if let repositoryID = repository.backendID {
                await withTaskGroup(of: (Int, [ActivityReaction]?).self) { group in
                    for index in fetched.indices {
                        // ロック中はリアクションを操作・表示しないため、追加API取得も行わない。
                        guard fetched[index].isLocked == false,
                              let postID = fetched[index].backendPostID else { continue }
                        group.addTask {
                            let reactions = try? await self.repositoryAPI.listReactions(
                                repositoryID: repositoryID,
                                postID: postID,
                                currentUserID: currentUserID,
                                accessToken: accessToken
                            )
                            return (index, reactions)
                        }
                    }

                    for await (index, reactions) in group {
                        if let reactions {
                            fetched[index].reactions = reactions
                        }
                    }
                }
            }

            activities = fetched
        } catch {
            errorMessage = "投稿を取得できませんでした"
        }

        //  フィード取得に失敗しても、BeGit Timeの表示取得は継続する。
        await refreshActiveChallenge(accessToken: accessToken)
    }

    func refreshActiveChallenge(accessToken: String?) async {
        guard let accessToken, let repositoryID = repository.backendID else {
            activeBeGitTime = nil
            activeChallenge = nil
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
            } else if NotificationDeliveryMode.current.usesLocalNotificationMock,
                      let cached = ActiveBeGitTimeStore.load(repositoryID: repositoryID) {
                // ローカルモック時だけ端末内のBeGit Timeを表示する。
                activeBeGitTime = cached
            } else {
                activeBeGitTime = nil
                ActiveBeGitTimeStore.remove(repositoryID: repositoryID)
                if endedBeGitTime == nil {
                    notificationMemberStatuses = []
                }
            }
        } catch {
            // 実APIモードでは、通知IDを持つ送信直後のキャッシュだけをフォールバックに使う。
            // 旧ローカルモックの notificationID=0 を復元すると、存在しない通知の停止を
            // 試みてしまうため除外する。
            let cached = ActiveBeGitTimeStore.load(repositoryID: repositoryID)
            activeBeGitTime = cached.flatMap { cached in
                NotificationDeliveryMode.current.usesLocalNotificationMock || cached.notificationID > 0
                    ? cached
                    : nil
            }
            if activeBeGitTime == nil && endedBeGitTime == nil {
                notificationMemberStatuses = []
            }
        }

        //  GitHub活動によって作成された下書き（active_challenge.my_post）も
        //  同じ更新タイミングで再取得する。これがないと、commit後も
        //  「撮影して投稿」が表示されないままになる。
        await loadActiveChallenge(accessToken: accessToken)
    }

    func stopActiveBeGitTime(accessToken: String?) async throws {
        guard let repositoryID = repository.backendID else {
            throw BeGitAPIError.invalidResponse
        }

        //  active_challengeを正の状態管理元にしつつ、旧active APIの
        //  レスポンスがまだ取れない場合も停止できるようにする。
        let active: ActiveBeGitTime
        if let activeBeGitTime {
            active = activeBeGitTime
        } else if let challenge = activeChallenge {
            active = ActiveBeGitTime(
                notificationID: challenge.notificationID,
                sentBy: challenge.issuer.userID,
                sentAt: challenge.sentAt,
                expiresAt: challenge.endsAt
            )
        } else {
            throw BeGitAPIError.invalidResponse
        }

        // Debugのローカル通知モードにはバックエンド通知IDがないため、
        // サーバーAPIではなく端末内のBeGit Timeを停止する。
        if NotificationDeliveryMode.current.usesLocalNotificationMock {
            endedBeGitTime = active
            activeBeGitTime = nil
            activeChallenge = nil
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
        activeChallenge = nil
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

}
