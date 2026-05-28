//  RepositoryDashboardViewModel.swift
//  Repository Dashboard画面の状態管理

import Foundation
import Combine

@MainActor
final class RepositoryDashboardViewModel: ObservableObject {
    let repository: Repository                                      //  表示対象Repository
    @Published private(set) var activities: [RepositoryActivity]    //  Timeline表示用activity一覧

    init(
        repository: Repository,
        activities: [RepositoryActivity]? = nil
    ) {
        self.repository = repository
        //  activity未指定時はMock dataを利用
        self.activities = activities ?? RepositoryActivity.mockActivities(for: repository)
    }
}

