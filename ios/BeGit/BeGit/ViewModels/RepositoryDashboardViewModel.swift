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
    }
}
