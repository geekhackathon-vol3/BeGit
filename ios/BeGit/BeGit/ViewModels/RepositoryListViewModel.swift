//  RepositoryListViewModel.swift
//  Repository一覧画面の状態管理

import Foundation
import Combine

@MainActor
final class RepositoryListViewModel: ObservableObject {
    @Published private(set) var repositories: [Repository]  //  表示中のRepository一覧
    @Published var isShowingAddRepository = false           //  Repository追加画面の表示状態
    @Published private(set) var isLoading = false            //  一覧取得中
    @Published var errorMessage: String?                     //  APIエラー表示
    @Published private(set) var isAuthExpired = false        //  認証期限切れ（401）

    private let repositoryAPI: any RepositoryAPI

    init(
        repositories: [Repository] = [],
        repositoryAPI: any RepositoryAPI = BeGitBackendAPI()
    ) {
        self.repositories = repositories
        self.repositoryAPI = repositoryAPI
    }

    // MARK: - Actions

    func loadRepositories(accessToken: String?) async {
        guard let accessToken else {
            repositories = []
            errorMessage = nil
            isLoading = false
            return
        }

        if shouldUseMockGitHubAPI(accessToken: accessToken) {
            repositories = Repository.mockRepositories
            errorMessage = nil
            isLoading = false
            return
        }

        isLoading = true
        errorMessage = nil
        isAuthExpired = false
        defer { isLoading = false }

        do {
            repositories = try await repositoryAPI.listRepositories(accessToken: accessToken)
        } catch let error as BeGitAPIError {
            switch error {
            case .requestFailed(statusCode: 401, _):
                isAuthExpired = true
                repositories = []
            default:
                errorMessage = error.localizedDescription
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    //  Repository追加画面を表示
    func showAddRepository() {
        isShowingAddRepository = true
    }

    //  Repositoryを一覧へ追加
    func addRepository(_ repository: Repository) {
        repositories.insert(repository, at: 0)
    }

    //  Repositoryを一覧から削除
    func removeRepository(_ repository: Repository) {
        repositories.removeAll { $0.id == repository.id }
    }
}
