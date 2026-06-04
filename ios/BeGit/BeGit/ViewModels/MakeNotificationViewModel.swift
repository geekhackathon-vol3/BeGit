//  MakeNotificationViewModel.swift
//  通知作成画面のローカル入力状態管理

import Foundation
import Combine

@MainActor
final class MakeNotificationViewModel: ObservableObject {
    let repository: Repository                      //  通知対象Repository

    @Published var members: [RepositoryMember]      //  Repository member一覧
    @Published private(set) var repositoryMemberCandidates: [RepositoryMember] // Repository由来のmember候補一覧
    @Published var selectedMemberIDs: Set<UUID>     //  選択中member ID一覧
    @Published var comment = ""                     //  通知コメント入力値
    @Published private(set) var isSending = false   //  通知送信中
    @Published private(set) var isLoadingMembers = false // member同期中
    @Published var errorMessage: String?            //  APIエラー表示

    private let repositoryAPI: any RepositoryAPI    // Repository関連API

    init(repository: Repository, repositoryAPI: any RepositoryAPI = BeGitBackendAPI()) {
        self.repository = repository
        self.members = repository.members                           //  初期member一覧
        self.repositoryMemberCandidates = repository.members         //  初期Repository member候補一覧
        self.selectedMemberIDs = Set(repository.members.map(\.id))  //  初期状態では全memberを選択
        self.repositoryAPI = repositoryAPI
    }

    //  管理者member
    var adminMember: RepositoryMember? {
        members.first
    }

    //  選択中member一覧
    var selectedMembers: [RepositoryMember] {
        members.filter { selectedMemberIDs.contains($0.id) }
    }

    //  通知送信可能か
    var canSend: Bool {
        selectedMembers.isEmpty == false && isSending == false
    }

    // MARK: - Actions

    //  member選択状態切り替え
    func toggleSelection(for member: RepositoryMember) {
        if selectedMemberIDs.contains(member.id) {
            //  選択解除
            selectedMemberIDs.remove(member.id)
        } else {
            //  選択追加
            selectedMemberIDs.insert(member.id)
        }
    }

    //  member削除
    func removeMember(_ member: RepositoryMember) {
        members.removeAll { $0.id == member.id }
        //  選択状態からも削除
        selectedMemberIDs.remove(member.id)
    }

    //  GitHub検索結果からmember追加
    func addMember(_ member: RepositoryMember) {
        guard members.contains(where: { $0.login.caseInsensitiveCompare(member.login) == .orderedSame }) == false else {
            return
        }

        members.append(member)
        selectedMemberIDs.insert(member.id)
    }

    //  通知モデル生成
    func makeNotification() -> RepositoryNotification {
        RepositoryNotification(
            repository: repository,
            //  選択中memberのみ通知対象
            selectedMembers: selectedMembers,
            //  前後空白を除去
            comment: comment.trimmingCharacters(in: .whitespacesAndNewlines)
        )
    }

    func loadMembers(accessToken: String?) async {
        guard isLoadingMembers == false else { return }
        guard let accessToken, let backendID = repository.backendID else { return }

        isLoadingMembers = true
        errorMessage = nil
        defer { isLoadingMembers = false }

        do {
            let syncedRepository = try await repositoryAPI.getRepository(id: backendID, accessToken: accessToken)
            members = syncedRepository.members
            repositoryMemberCandidates = syncedRepository.members
            selectedMemberIDs = Set(syncedRepository.members.map(\.id))
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func sendNotification(accessToken: String?) async -> RepositoryNotification? {
        guard isSending == false else { return nil }
        guard let accessToken else {
            errorMessage = "アクセストークンが見つかりません。"
            return nil
        }
        guard let backendID = repository.backendID else {
            errorMessage = "Repository IDが見つかりません。"
            return nil
        }

        isSending = true
        errorMessage = nil
        defer { isSending = false }

        do {
            try await repositoryAPI.sendNotification(repositoryID: backendID, accessToken: accessToken)
            return makeNotification()
        } catch {
            errorMessage = error.localizedDescription
            return nil
        }
    }
}
