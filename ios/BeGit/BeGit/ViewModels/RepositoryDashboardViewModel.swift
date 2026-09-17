//  RepositoryDashboardViewModel.swift
//  Repository Dashboard画面の状態管理

import Foundation
import Combine

@MainActor
final class RepositoryDashboardViewModel: ObservableObject {
    let repository: Repository                                      //  表示対象Repository
    @Published private(set) var activities: [RepositoryActivity]    //  Timeline表示用activity一覧
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
}
