//  AddRepositoryViewModel.swift
//  Repository追加画面の入力状態管理

import Foundation
import Combine

@MainActor
final class AddRepositoryViewModel: ObservableObject {
    @Published var repositoryURLText = ""                                      // Repository URL入力値
    @Published var repositorySearchText = ""                                   // Repository検索入力値

    @Published private(set) var availableRepositories: [GitHubRepository] = []  // GitHub Repository候補一覧
    @Published private(set) var selectedRepository: GitHubRepository?           // 選択中Repository
    @Published private(set) var isLoadingRepositories = false                   // Repository一覧取得中か
    @Published private(set) var repositoryListErrorMessage: String?             // Repository一覧取得エラー
    @Published private(set) var visibleRepositoryCount = 3                      // 画面に表示するRepository候補数

    @Published var memberLoginText = ""                                         // 招待するmember login入力値
    @Published private(set) var members: [RepositoryMember] = []                // 追加済みmember一覧
    @Published private(set) var repositoryMemberCandidates: [RepositoryMember] = [] // Repository由来のmember候補一覧
    @Published private(set) var isLoadingMembers = false                        // member同期中か
    @Published private(set) var memberListErrorMessage: String?                 // member同期エラー
    @Published var isMemberInputVisible = false                                 // member入力欄の表示状態

    @Published private(set) var isSaving = false                                // Repository作成API実行中
    @Published var errorMessage: String?                                        // エラーメッセージ

    let invitedMembers: [RepositoryMember] = [                                  // 招待済みmember候補一覧
        RepositoryMember(login: "ayaka"),
        RepositoryMember(login: "begit"),
        RepositoryMember(login: "ios-dev"),
        RepositoryMember(login: "repo-admin")
    ]

    private let accessToken: String?
    private let backendRepositoryAPI: any RepositoryAPI                         // BeGit Repository関連API
    private let githubRepositoryAPI: any GitHubRepositoryAPI                    // GitHub Repository一覧API

    init(
        accessToken: String? = nil,
        backendRepositoryAPI: any RepositoryAPI = BeGitBackendAPI(),
        githubRepositoryAPI: (any GitHubRepositoryAPI)? = nil
    ) {
        self.accessToken = accessToken
        self.backendRepositoryAPI = backendRepositoryAPI

        if let githubRepositoryAPI {
            self.githubRepositoryAPI = githubRepositoryAPI
        } else if accessToken?.hasPrefix("mock_access_token_") == true {
            self.githubRepositoryAPI = MockGitHubRepositoryAPI()
        } else {
            self.githubRepositoryAPI = GitHubRepositoryClient()
        }
    }

    //  Repository作成可能か
    var canComplete: Bool {
        repositoryName != nil && isSaving == false
    }

    //  Repository preview表示名
    var repositoryPreviewName: String? {
        repositoryName
    }

    //  画面に表示するRepository候補
    var displayedRepositories: [GitHubRepository] {
        Array(filteredRepositories.prefix(visibleRepositoryCount))
    }

    //  追加表示できるRepository候補があるか
    var canShowMoreRepositories: Bool {
        visibleRepositoryCount < filteredRepositories.count
    }

    //  member追加可能か
    var canAddMember: Bool {
        normalizedMemberLogin.isEmpty == false
            && members.contains { $0.login.caseInsensitiveCompare(normalizedMemberLogin) == .orderedSame } == false
    }

    //  未追加の招待済みmember
    var selectableInvitedMembers: [RepositoryMember] {
        invitedMembers.filter { invitedMember in
            members.contains { $0.login.caseInsensitiveCompare(invitedMember.login) == .orderedSame } == false
        }
    }

    // MARK: - Actions

    func updateRepositorySearchText(_ text: String) {
        repositorySearchText = text
        visibleRepositoryCount = min(3, filteredRepositories.count)
    }

    //  GitHub Repository一覧を取得
    func loadRepositories() async {
        guard availableRepositories.isEmpty, isLoadingRepositories == false else {
            return
        }

        guard let accessToken, accessToken.isEmpty == false else {
            repositoryListErrorMessage = "GitHubログイン情報を取得できませんでした。再ログインしてください。"
            return
        }

        isLoadingRepositories = true
        repositoryListErrorMessage = nil

        do {
            availableRepositories = try await githubRepositoryAPI.listRepositories(accessToken: accessToken)
            visibleRepositoryCount = min(3, availableRepositories.count)
        } catch {
            repositoryListErrorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
        }

        isLoadingRepositories = false
    }

    //  Repository候補を選択
    func selectRepository(_ repository: GitHubRepository) async {
        selectedRepository = repository
        repositoryURLText = "https://github.com/\(repository.fullName)"
        await loadRepositoryMembers(repoFullName: repository.fullName)
    }

    //  Repository候補を追加表示
    func showMoreRepositories() {
        visibleRepositoryCount = min(visibleRepositoryCount + 3, availableRepositories.count)
    }

    //  member選択リスト表示を切り替え
    func showMemberInput() {
        isMemberInputVisible.toggle()
    }

    //  member追加
    func addMember() {
        guard canAddMember else { return }

        members.append(RepositoryMember(login: normalizedMemberLogin))
        memberLoginText = ""
        isMemberInputVisible = false
    }

    //  GitHub検索結果からmember追加
    func addMember(_ member: RepositoryMember) {
        guard members.contains(where: { $0.login.caseInsensitiveCompare(member.login) == .orderedSame }) == false else {
            return
        }

        members.append(member)
    }

    //  招待済みmember追加
    func addInvitedMember(_ member: RepositoryMember) {
        guard members.contains(where: { $0.login.caseInsensitiveCompare(member.login) == .orderedSame }) == false else {
            return
        }

        members.append(member)
    }

    //  member削除
    func removeMember(_ member: RepositoryMember) {
        members.removeAll { $0.id == member.id }
    }

    //  入力値からRepository生成
    func createRepository(accessToken: String?) async -> Repository? {
        guard isSaving == false else { return nil }
        guard let repositoryName, let accessToken else { return nil }

        isSaving = true
        errorMessage = nil
        defer { isSaving = false }

        do {
            let createdRepository = try await backendRepositoryAPI.createRepository(
                repoFullName: repositoryName,
                name: repositoryName,
                accessToken: accessToken
            )
            return repositoryWithSelectedOwnerAvatar(createdRepository)
        } catch {
            errorMessage = error.localizedDescription
            return nil
        }
    }

    // MARK: - Private

    //  検索条件に一致するRepository候補
    private var filteredRepositories: [GitHubRepository] {
        let query = repositorySearchText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard query.isEmpty == false else {
            return availableRepositories
        }

        return availableRepositories.filter { repository in
            repository.fullName.localizedCaseInsensitiveContains(query)
                || (repository.description?.localizedCaseInsensitiveContains(query) ?? false)
        }
    }

    //  選択RepositoryのGitHub collaboratorを取得
    private func loadRepositoryMembers(repoFullName: String) async {
        guard let accessToken, accessToken.isEmpty == false else {
            memberListErrorMessage = "GitHubログイン情報を取得できませんでした。再ログインしてください。"
            members = []
            repositoryMemberCandidates = []
            return
        }

        isLoadingMembers = true
        memberListErrorMessage = nil
        members = []
        repositoryMemberCandidates = []
        defer { isLoadingMembers = false }

        do {
            let repositoryMembers = try await githubRepositoryAPI.listRepositoryMembers(
                repoFullName: repoFullName,
                accessToken: accessToken
            )
            members = repositoryMembers
            repositoryMemberCandidates = repositoryMembers
        } catch {
            memberListErrorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
        }
    }

    //  前後空白を除去したmember login
    private var normalizedMemberLogin: String {
        memberLoginText.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    //  GitHub Repository選択時に取得済みのowner avatarを作成結果へ反映
    private func repositoryWithSelectedOwnerAvatar(_ repository: Repository) -> Repository {
        guard let selectedRepository,
              let ownerAvatarURL = selectedRepository.ownerAvatarURL,
              repository.ownerAvatarURL == nil else {
            return repository
        }

        return Repository(
            id: repository.id,
            backendID: repository.backendID,
            name: repository.name,
            ownerAvatarURL: ownerAvatarURL,
            memberCount: repository.memberCount,
            members: repository.members
        )
    }

    //  GitHub URLからRepository名を抽出
    private var repositoryName: String? {
        if let selectedRepository {
            return selectedRepository.fullName
        }

        let trimmedText = repositoryURLText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let url = URL(string: trimmedText), url.host?.lowercased() == "github.com" else {
            return nil
        }

        let pathComponents = url.pathComponents.filter { $0 != "/" }
        guard pathComponents.count >= 2 else { return nil }

        return "\(pathComponents[0])/\(pathComponents[1])"
    }
}
