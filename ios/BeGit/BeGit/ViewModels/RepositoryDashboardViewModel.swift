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
    @Published private(set) var notificationMessage = ""
    @Published private(set) var gameResult: BeGitGameResult = .pending
    @Published private(set) var isLoading = false                   //  Timeline取得中
    @Published var errorMessage: String?                            //  APIエラー表示
    //  進行中のBeGit Time（GET /groups/:id の active_challenge）。nil = 進行中なし
    @Published private(set) var activeChallenge: ActiveChallenge?

    @Published private(set) var hasMoreActivities = false
    @Published private(set) var isLoadingMore = false

    private let repositoryAPI: any RepositoryAPI
    private var nextActivityCursor: String?
    private var repositoryChangeCursor: Int64 = 0
    private var currentUserID: Int64?

    init(
        repository: Repository,
        activities: [RepositoryActivity]? = nil,
        activeChallenge: ActiveChallenge? = nil,
        repositoryAPI: any RepositoryAPI = BeGitBackendAPI()
    ) {
        self.repository = repository
        let cached = repository.backendID.flatMap { RepositoryDashboardCache.snapshots[$0] }
        self.activities = activities ?? cached?.activities ?? []
        self.activeChallenge = activeChallenge ?? cached?.activeChallenge ?? repository.activeChallenge
        self.activeBeGitTime = cached?.activeBeGitTime
        self.notificationMemberStatuses = cached?.memberStatuses ?? []
        self.notificationMessage = cached?.notificationMessage ?? ""
        self.gameResult = cached?.gameResult ?? .pending
        self.nextActivityCursor = cached?.nextCursor
        self.hasMoreActivities = cached?.nextCursor != nil
        self.repositoryChangeCursor = cached?.changeCursor
            ?? repository.backendID.map(RepositoryChangeCursorStore.load)
            ?? 0
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
        self.currentUserID = currentUserID
        errorMessage = nil
        defer { isLoading = false }

        do {
            if repositoryChangeCursor == 0, let repositoryID = repository.backendID,
               let watermark = try? await repositoryAPI.listRepositoryChanges(
                    repositoryID: repositoryID, after: Int64.max, accessToken: accessToken
               ) {
                repositoryChangeCursor = watermark.cursor
                RepositoryChangeCursorStore.save(watermark.cursor, repositoryID: repositoryID)
            }
            let page = try await repositoryAPI.listActivityPage(
                repository: repository, cursor: nil, currentUserID: currentUserID, accessToken: accessToken
            )
            let fetched = page.activities
            if fetched != activities {
                activities = fetched
            }
            nextActivityCursor = page.nextCursor
            hasMoreActivities = page.hasMore
            saveCache()
        } catch {
            errorMessage = "投稿を取得できませんでした"
        }

        //  フィード取得に失敗しても、BeGit Timeの表示取得は継続する。
        await refreshActiveChallenge(accessToken: accessToken)
    }

    func loadMoreActivities(accessToken: String?) async {
        guard isLoadingMore == false, hasMoreActivities, let cursor = nextActivityCursor,
              let accessToken else { return }
        isLoadingMore = true
        defer { isLoadingMore = false }
        do {
            let page = try await repositoryAPI.listActivityPage(
                repository: repository, cursor: cursor, currentUserID: currentUserID, accessToken: accessToken
            )
            var known = Set(activities.compactMap(\.backendPostID))
            activities.append(contentsOf: page.activities.filter { activity in
                guard let id = activity.backendPostID else { return true }
                return known.insert(id).inserted
            })
            nextActivityCursor = page.nextCursor
            hasMoreActivities = page.hasMore
            saveCache()
        } catch {
            //  既存ページを維持する。
        }
    }

    //  他メンバーの新着投稿を定期反映する軽量更新。
    //  リアクションAPIの再取得は行わず、取得済みの状態を投稿IDごとに引き継ぐ。
    func refreshActivities(accessToken: String?) async {
        guard let accessToken, repository.backendID != nil else { return }

        do {
            let reactionsByPostID = activities.reduce(into: [Int64: [ActivityReaction]]()) {
                result, activity in
                guard let postID = activity.backendPostID else { return }
                result[postID] = activity.reactions
            }
            var fetched = try await repositoryAPI.listActivities(
                repository: repository,
                accessToken: accessToken
            )

            for index in fetched.indices {
                guard let postID = fetched[index].backendPostID,
                      let reactions = reactionsByPostID[postID] else { continue }
                fetched[index].reactions = reactions
            }

            let merged = RepositoryActivity.mergingRefresh(fetched, preserving: activities)
            if merged != activities {
                activities = merged
            }
        } catch {
            //  バックグラウンド更新失敗時は、現在表示中の投稿を維持する。
        }
    }

    // Repository change logから変更された投稿だけをupsert/deleteする。
    func refreshRepositoryChanges(accessToken: String?) async {
        guard let accessToken, let repositoryID = repository.backendID else { return }
        do {
            repeat {
                let batch = try await repositoryAPI.listRepositoryChanges(
                    repositoryID: repositoryID, after: repositoryChangeCursor, accessToken: accessToken
                )
                if batch.resetRequired {
                    repositoryChangeCursor = batch.cursor
                    RepositoryChangeCursorStore.save(batch.cursor, repositoryID: repositoryID)
                    await loadActivities(accessToken: accessToken, currentUserID: currentUserID)
                    saveCache()
                    return
                }

                var latestPostOperations: [Int64: String] = [:]
                var challengeChanged = false
                for change in batch.changes {
                    if let postID = change.postID {
                        latestPostOperations[postID] = change.operation
                    } else {
                        challengeChanged = true
                    }
                }
                for (postID, operation) in latestPostOperations {
                    if operation == "delete" {
                        activities.removeAll { $0.backendPostID == postID }
                    } else if let refreshed = try await repositoryAPI.getActivity(
                        repository: repository, postID: postID, currentUserID: currentUserID, accessToken: accessToken
                    ) {
                        if let index = activities.firstIndex(where: { $0.backendPostID == postID }) {
                            activities[index] = refreshed.preservingClientState(
                                from: activities[index], preservingReactions: false
                            )
                        } else {
                            activities.append(refreshed)
                        }
                    } else {
                        activities.removeAll { $0.backendPostID == postID }
                    }
                }
                activities.sort { lhs, rhs in
                    lhs.date == rhs.date ? String(describing: lhs.id) > String(describing: rhs.id) : lhs.date > rhs.date
                }
                repositoryChangeCursor = batch.cursor
                RepositoryChangeCursorStore.save(batch.cursor, repositoryID: repositoryID)
                if challengeChanged { await refreshActiveChallenge(accessToken: accessToken) }
                saveCache()
                if batch.hasMore == false { break }
            } while Task.isCancelled == false
        } catch {
            // FCM・pollはヒントなので、失敗時はキャッシュを表示したまま次回再試行する。
        }
    }

    func refreshActiveChallenge(accessToken: String?) async {
        let previousNotificationID = activeChallenge?.notificationID ?? activeBeGitTime?.notificationID
        guard let accessToken, let repositoryID = repository.backendID else {
            activeBeGitTime = nil
            activeChallenge = nil
            notificationMemberStatuses = []
            notificationMessage = ""
            gameResult = .pending
            return
        }

        do {
            //  GET /groups/:id の active_challenge を状態管理元にする。
            //  /notifications/active を重ねて呼ばないことで、1回のポーリングを
            //  Repository取得 + 必要時のstatus取得の2通信に抑える。
            let synced = try await repositoryAPI.getRepository(id: repositoryID, accessToken: accessToken)
            if let challenge = synced.activeChallenge {
                let active = ActiveBeGitTime(
                    notificationID: challenge.notificationID,
                    sentBy: challenge.issuer.userID,
                    sentAt: challenge.sentAt,
                    expiresAt: challenge.endsAt,
                    gameMode: challenge.gameMode
                )
                let notificationChanged = previousNotificationID != challenge.notificationID
                if activeChallenge != challenge {
                    activeChallenge = challenge
                }
                if activeBeGitTime != active { activeBeGitTime = active }
                if endedBeGitTime != nil {
                    endedBeGitTime = nil
                }
                if notificationChanged {
                    resetNotificationStatus()
                }
                if let statuses = try? await repositoryAPI.getNotificationStatus(
                    repositoryID: repositoryID,
                    notificationID: challenge.notificationID,
                    accessToken: accessToken
                ) {
                    applyNotificationStatus(statuses)
                }
            } else if NotificationDeliveryMode.current.usesLocalNotificationMock,
                      let cached = ActiveBeGitTimeStore.load(repositoryID: repositoryID) {
                // ローカルモック時だけ端末内のBeGit Timeを表示する。
                if activeBeGitTime != cached {
                    activeBeGitTime = cached
                }
                activeChallenge = nil
                if previousNotificationID != cached.notificationID {
                    resetNotificationStatus()
                }
            } else {
                if activeBeGitTime != nil {
                    activeBeGitTime = nil
                }
                if activeChallenge != nil {
                    activeChallenge = nil
                }
                ActiveBeGitTimeStore.remove(repositoryID: repositoryID)
                if endedBeGitTime == nil {
                    resetNotificationStatus()
                }
            }
        } catch {
            // 実APIモードでは、通知IDを持つ送信直後のキャッシュだけをフォールバックに使う。
            // 旧ローカルモックの notificationID=0 を復元すると、存在しない通知の停止を
            // 試みてしまうため除外する。
            let cached = ActiveBeGitTimeStore.load(repositoryID: repositoryID)
            let fallback = cached.flatMap { cached in
                NotificationDeliveryMode.current.usesLocalNotificationMock || cached.notificationID > 0
                    ? cached
                    : nil
            }
            if activeBeGitTime != fallback {
                activeBeGitTime = fallback
            }
            if activeBeGitTime?.notificationID != previousNotificationID {
                resetNotificationStatus()
            }
            if activeBeGitTime == nil && endedBeGitTime == nil {
                resetNotificationStatus()
            }
        }
        saveCache()

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
                expiresAt: challenge.endsAt,
                gameMode: challenge.gameMode
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
            notificationMessage = ""
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
            applyNotificationStatus(statuses)
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
        activityID: RepositoryActivity.ID,
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

    private func updateReactions(_ reactions: [ActivityReaction], for activityID: RepositoryActivity.ID) {
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
            if activeChallenge != synced.activeChallenge {
                activeChallenge = synced.activeChallenge
            }
        } catch {
            //  失敗しても既存表示を維持
        }
    }

    private func applyNotificationStatus(_ status: NotificationStatusSummary) {
        if notificationMemberStatuses != status.members {
            notificationMemberStatuses = status.members
        }
        if gameResult != status.gameResult {
            gameResult = status.gameResult
        }
        if notificationMessage != status.message {
            notificationMessage = status.message
        }
    }

    private func resetNotificationStatus() {
        if notificationMemberStatuses.isEmpty == false {
            notificationMemberStatuses = []
        }
        if notificationMessage.isEmpty == false {
            notificationMessage = ""
        }
        if gameResult != .pending {
            gameResult = .pending
        }
    }

    private func saveCache() {
        guard let repositoryID = repository.backendID else { return }
        RepositoryDashboardCache.snapshots[repositoryID] = RepositoryDashboardSnapshot(
            activities: activities,
            activeChallenge: activeChallenge,
            activeBeGitTime: activeBeGitTime,
            memberStatuses: notificationMemberStatuses,
            notificationMessage: notificationMessage,
            gameResult: gameResult,
            nextCursor: nextActivityCursor,
            changeCursor: repositoryChangeCursor
        )
    }

}

private struct RepositoryDashboardSnapshot {
    let activities: [RepositoryActivity]
    let activeChallenge: ActiveChallenge?
    let activeBeGitTime: ActiveBeGitTime?
    let memberStatuses: [NotificationMemberStatus]
    let notificationMessage: String
    let gameResult: BeGitGameResult
    let nextCursor: String?
    let changeCursor: Int64
}

@MainActor
private enum RepositoryDashboardCache {
    static var snapshots: [Int64: RepositoryDashboardSnapshot] = [:]
}

private enum RepositoryChangeCursorStore {
    private static let prefix = "begit.repository-change-cursor"

    static func load(repositoryID: Int64) -> Int64 {
        Int64(UserDefaults.standard.string(forKey: "\(prefix).\(repositoryID)") ?? "") ?? 0
    }

    static func save(_ cursor: Int64, repositoryID: Int64) {
        UserDefaults.standard.set(String(cursor), forKey: "\(prefix).\(repositoryID)")
    }
}
