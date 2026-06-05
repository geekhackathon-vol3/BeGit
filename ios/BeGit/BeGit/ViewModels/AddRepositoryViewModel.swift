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
    private let existingRepositoryNames: Set<String>
    private let backendRepositoryAPI: any RepositoryAPI                         // BeGit Repository関連API
    private let githubRepositoryAPI: any GitHubRepositoryAPI                    // GitHub Repository一覧API

    init(
        accessToken: String? = nil,
        existingRepositories: [Repository] = [],
        backendRepositoryAPI: any RepositoryAPI = BeGitBackendAPI(),
        githubRepositoryAPI: (any GitHubRepositoryAPI)? = nil
    ) {
        self.accessToken = accessToken
        self.existingRepositoryNames = Set(existingRepositories.map { Self.normalizedRepositoryName($0.name) })
        self.backendRepositoryAPI = backendRepositoryAPI

        if let githubRepositoryAPI {
            self.githubRepositoryAPI = githubRepositoryAPI
        } else if shouldUseMockGitHubAPI(accessToken: accessToken) {
            self.githubRepositoryAPI = MockGitHubRepositoryAPI()
        } else {
            self.githubRepositoryAPI = GitHubRepositoryClient()
        }
    }

    //  Repository作成可能か
    var canComplete: Bool {
        guard let repositoryName else { return false }

        return isSaving == false && isRepositoryAlreadyAdded(repositoryName) == false
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

    func isAlreadyAdded(_ repository: GitHubRepository) -> Bool {
        isRepositoryAlreadyAdded(repository.fullName)
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
            let fetched = try await githubRepositoryAPI.listRepositories(accessToken: accessToken)
            //  発表用デモリポジトリを先頭に固定
            availableRepositories = [Self.presentationRepo] + fetched
            visibleRepositoryCount = min(3, availableRepositories.count)
        } catch {
            availableRepositories = [Self.presentationRepo]
            visibleRepositoryCount = 1
            repositoryListErrorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
        }

        isLoadingRepositories = false
    }

    //  Repository候補を選択
    func selectRepository(_ repository: GitHubRepository) async {
        if isAlreadyAdded(repository) {
            reportAlreadyAdded(repository.fullName)
            return
        }

        errorMessage = nil
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

        guard isRepositoryAlreadyAdded(repositoryName) == false else {
            reportAlreadyAdded(repositoryName)
            return nil
        }

        isSaving = true
        errorMessage = nil
        defer { isSaving = false }

        if repositoryName == Self.presentationRepo.fullName {
            //  デモリポジトリ: ダミーbackendIDを付与してカメラを開けるようにする
            let base = makeLocalRepository(name: repositoryName)
            return Repository(
                backendID: -1,
                name: base.name,
                ownerAvatarURL: base.ownerAvatarURL,
                memberCount: base.memberCount,
                members: base.members
            )
        }

        if shouldUseMockGitHubAPI(accessToken: accessToken) {
            return makeLocalRepository(name: repositoryName)
        }

        do {
            let createdRepository = try await backendRepositoryAPI.createRepository(
                repoFullName: repositoryName,
                name: repositoryName,
                accessToken: accessToken
            )
            return repositoryWithSelectedOwnerAvatar(createdRepository)
        } catch {
            if isConflictError(error),
               let existingRepository = await restoreExistingRepository(
                repositoryName: repositoryName,
                accessToken: accessToken
               ) {
                return existingRepository
            }

            if isBackendUnavailable(error) || shouldUseMockGitHubAPI(accessToken: accessToken) {
                return makeLocalRepository(name: repositoryName)
            }

            errorMessage = error.localizedDescription
            return nil
        }
    }

    // MARK: - Private

    //  発表用デモメンバー
    private static let presentationMembers: [RepositoryMember] = [
        RepositoryMember(login: "Riochin",     avatarURL: URL(string: "https://avatars.githubusercontent.com/u/175614867?v=4")),
        RepositoryMember(login: "s2108tomoka", avatarURL: URL(string: "https://avatars.githubusercontent.com/u/163800046?v=4")),
        RepositoryMember(login: "liruly",      avatarURL: URL(string: "https://avatars.githubusercontent.com/u/141731612?v=4")),
        RepositoryMember(login: "palm7710",    avatarURL: URL(string: "https://avatars.githubusercontent.com/u/168710387?v=4")),
    ]

    //  発表用デモリポジトリ（常に先頭に固定表示）
    private static let presentationRepo = GitHubRepository(
        id: -1,
        fullName: "hackathon/BeGit",
        description: "チームのGitHub活動を可視化するアプリ",
        isPrivate: false,
        ownerAvatarURL: URL(string: "https://avatars.githubusercontent.com/u/168710387?v=4"),
        updatedAt: nil
    )

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
        //  発表用デモリポジトリはモックメンバーを直接返す
        if repoFullName == Self.presentationRepo.fullName {
            let demoMembers = Self.presentationMembers
            members = demoMembers
            repositoryMemberCandidates = demoMembers
            return
        }

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

    private func isRepositoryAlreadyAdded(_ repositoryName: String) -> Bool {
        existingRepositoryNames.contains(Self.normalizedRepositoryName(repositoryName))
    }

    private func reportAlreadyAdded(_ repositoryName: String) {
        errorMessage = "\(repositoryName) はすでに追加されています。"
    }

    private func isConflictError(_ error: Error) -> Bool {
        if case let BeGitAPIError.requestFailed(statusCode, _) = error {
            return statusCode == 409
        }

        let errorText = "\(String(describing: error)) \(error.localizedDescription)".lowercased()
        if errorText.contains("409") && errorText.contains("conflict") {
            return true
        }

        if let underlyingError = (error as NSError).userInfo[NSUnderlyingErrorKey] as? Error,
           isConflictError(underlyingError) {
            return true
        }

        for child in Mirror(reflecting: error).children {
            if let childError = errorValue(from: child.value),
               isConflictError(childError) {
                return true
            }
        }

        return false
    }

    private func errorValue(from value: Any) -> Error? {
        if let error = value as? Error {
            return error
        }

        let mirror = Mirror(reflecting: value)
        guard mirror.displayStyle == .optional,
              let wrappedValue = mirror.children.first?.value else {
            return nil
        }

        return errorValue(from: wrappedValue)
    }

    private func isBackendUnavailable(_ error: Error) -> Bool {
        let nsError = error as NSError
        if nsError.domain == NSURLErrorDomain {
            switch nsError.code {
            case NSURLErrorCannotConnectToHost,
                 NSURLErrorCannotFindHost,
                 NSURLErrorTimedOut,
                 NSURLErrorNetworkConnectionLost,
                 NSURLErrorNotConnectedToInternet,
                 NSURLErrorInternationalRoamingOff,
                 NSURLErrorDataNotAllowed,
                 NSURLErrorSecureConnectionFailed:
                return true
            default:
                break
            }
        }

        let message = nsError.localizedDescription.lowercased()
        return message.contains("could not connect")
            || message.contains("connection refused")
            || message.contains("timed out")
            || message.contains("connection lost")
            || message.contains("not connected to the internet")
    }

    private func restoreExistingRepository(repositoryName: String, accessToken: String) async -> Repository? {
        do {
            let repositories = try await backendRepositoryAPI.listRepositories(accessToken: accessToken)
            guard let existingRepository = repositories.first(where: {
                Self.normalizedRepositoryName($0.name) == Self.normalizedRepositoryName(repositoryName)
            }) else {
                errorMessage = "\(repositoryName) はすでに登録されています。一覧を更新して確認してください。"
                return nil
            }

            errorMessage = nil
            return repositoryWithSelectedOwnerAvatar(existingRepository)
        } catch {
            errorMessage = "\(repositoryName) はすでに登録されています。一覧を更新して確認してください。"
            return nil
        }
    }

    private static func normalizedRepositoryName(_ repositoryName: String) -> String {
        repositoryName.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    }

    //  Dev / Mock時はバックエンドへ未登録トークンを送らず、画面上へ即時追加する
    private func makeLocalRepository(name: String) -> Repository {
        Repository(
            name: name,
            ownerAvatarURL: selectedRepository?.ownerAvatarURL ?? ownerAvatarURL(from: name),
            memberCount: members.count,
            members: members
        )
    }

    private func ownerAvatarURL(from repositoryName: String) -> URL? {
        guard let owner = repositoryName.split(separator: "/").first else { return nil }

        return URL(string: "https://github.com/\(owner).png")
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
