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
            activities = try await repositoryAPI.listActivities(repository: repository, accessToken: accessToken)
        } catch {
            activities = RepositoryActivity.mockActivities(for: repository)
        }
    }
}
