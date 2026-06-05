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

    //  activityを投稿したmember数（達成済み）
    var completedCount: Int {
        Set(activities.map(\.author.login)).count
    }

    //  リポジトリ総member数（members未取得時はactivity数にフォールバック）
    var totalCount: Int {
        let memberCount = repository.members.count
        return memberCount > 0 ? memberCount : max(completedCount, 1)
    }

    //  達成率
    var progress: Double {
        Double(min(completedCount, totalCount)) / Double(totalCount)
    }

    //  達成状況テキスト
    var progressText: String {
        "\(min(completedCount, totalCount))/\(totalCount)人が達成しました"
    }

    func loadActivities(accessToken: String?) async {
        guard let accessToken else {
            activities = RepositoryActivity.mockActivities(for: repository)
            return
        }

        guard repository.backendID != nil else {
            activities = RepositoryActivity.mockActivities(for: repository)
            return
        }

        isLoading = true
        errorMessage = nil
        defer { isLoading = false }

        do {
            let fetched = try await repositoryAPI.listActivities(repository: repository, accessToken: accessToken)
            activities = fetched.isEmpty
                ? RepositoryActivity.mockActivities(for: repository)
                : fetched
        } catch {
            activities = RepositoryActivity.mockActivities(for: repository)
        }
    }
}
