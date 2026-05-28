//  RepositoryListViewModel.swift
//  Repository一覧画面の状態管理

import Foundation
import Combine

@MainActor
final class RepositoryListViewModel: ObservableObject {
    @Published private(set) var repositories: [Repository]  //  表示中のRepository一覧
    @Published var isShowingAddRepository = false           //  Repository追加画面の表示状態

    init(repositories: [Repository] = Repository.mockRepositories) {
        self.repositories = repositories
    }

    // MARK: - Actions

    //  Repository追加画面を表示
    func showAddRepository() {
        isShowingAddRepository = true
    }

    //  Repositoryを一覧へ追加
    func addRepository(_ repository: Repository) {
        repositories.insert(repository, at: 0)
    }
}
