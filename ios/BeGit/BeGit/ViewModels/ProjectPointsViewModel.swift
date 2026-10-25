import Foundation
import Combine

@MainActor
final class ProjectPointsViewModel: ObservableObject {
    let repository: Repository
    @Published private(set) var summary: ProjectPointsSummary?
    @Published private(set) var isLoading = false
    @Published private(set) var errorMessage: String?

    private let pointsAPI: any ProjectPointsAPI

    init(
        repository: Repository,
        initialFinalResult: ProjectFinalResult? = nil,
        pointsAPI: any ProjectPointsAPI = BeGitBackendAPI()
    ) {
        self.repository = repository
        self.summary = initialFinalResult.map(ProjectPointsSummary.init(finalResult:))
        self.pointsAPI = pointsAPI
    }

    var finalResult: ProjectFinalResult? { summary?.finalResult }

    func load(accessToken: String?) async {
        guard let repositoryID = repository.backendID,
              let accessToken, accessToken.isEmpty == false else {
            errorMessage = "ポイントを取得するための情報が不足しています"
            return
        }

        isLoading = summary == nil
        errorMessage = nil
        defer { isLoading = false }
        do {
            summary = try await pointsAPI.getPointsSummary(
                repositoryID: repositoryID,
                accessToken: accessToken
            )
        } catch {
            if summary == nil {
                errorMessage = "ポイントを取得できませんでした"
            }
        }
    }
}
