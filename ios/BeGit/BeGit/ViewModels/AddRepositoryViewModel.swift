//  AddRepositoryViewModel.swift
//  Repository追加画面の入力状態管理

import Foundation
import Combine

@MainActor
final class AddRepositoryViewModel: ObservableObject {
    @Published var repositoryURLText = ""                               //  Repository URL入力値
    @Published private(set) var availableRepositories: [GitHubRepository] = [] //  GitHub Repository候補一覧
    @Published private(set) var selectedRepository: GitHubRepository?   //  選択中Repository
    @Published private(set) var isLoadingRepositories = false           //  Repository一覧取得中か
    @Published private(set) var repositoryListErrorMessage: String?     //  Repository一覧取得エラー
    @Published private(set) var visibleRepositoryCount = 3              //  画面に表示するRepository候補数
    @Published var memberLoginText = ""                                 //  member login入力値
    @Published private(set) var members: [RepositoryMember] = []        //  追加済みmember一覧
    @Published var isMemberInputVisible = false                         //  member入力欄の表示状態
    let invitedMembers: [RepositoryMember] = [                          //  招待済みmember候補一覧
        RepositoryMember(login: "ayaka"),
        RepositoryMember(login: "begit"),
        RepositoryMember(login: "ios-dev"),
        RepositoryMember(login: "repo-admin")
    ]

    private let accessToken: String?
    private let repositoryAPI: any GitHubRepositoryAPI

    init(
        accessToken: String? = nil,
        repositoryAPI: (any GitHubRepositoryAPI)? = nil
    ) {
        self.accessToken = accessToken

        if let repositoryAPI {
            self.repositoryAPI = repositoryAPI
        } else if accessToken?.hasPrefix("mock_access_token_") == true {
            self.repositoryAPI = MockGitHubRepositoryAPI()
        } else {
            self.repositoryAPI = GitHubRepositoryClient()
        }
    }

    //  Repository作成可能か
    var canComplete: Bool {
        repositoryName != nil
    }

    //  Repository preview表示名
    var repositoryPreviewName: String? {
        repositoryName
    }

    //  画面に表示するRepository候補
    var displayedRepositories: [GitHubRepository] {
        Array(availableRepositories.prefix(visibleRepositoryCount))
    }

    //  追加表示できるRepository候補があるか
    var canShowMoreRepositories: Bool {
        visibleRepositoryCount < availableRepositories.count
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
            availableRepositories = try await repositoryAPI.listRepositories(accessToken: accessToken)
            visibleRepositoryCount = min(3, availableRepositories.count)
        } catch {
            repositoryListErrorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
        }

        isLoadingRepositories = false
    }

    //  Repository候補を選択
    func selectRepository(_ repository: GitHubRepository) {
        selectedRepository = repository
        repositoryURLText = "https://github.com/\(repository.fullName)"
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
    func makeRepository() -> Repository? {
        guard let repositoryName else { return nil }

        return Repository(
            name: repositoryName,
            memberCount: members.count,
            members: members
        )
    }

    // MARK: - Private

    //  前後空白を除去したmember login
    private var normalizedMemberLogin: String {
        memberLoginText.trimmingCharacters(in: .whitespacesAndNewlines)
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
