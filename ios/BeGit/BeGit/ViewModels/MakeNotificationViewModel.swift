//  MakeNotificationViewModel.swift
//  通知作成画面のローカル入力状態管理

import Foundation
import Combine

@MainActor
final class MakeNotificationViewModel: ObservableObject {
    let repository: Repository                      //  通知対象Repository
    let adminMember: RepositoryMember?              //  管理者member

    @Published var members: [RepositoryMember]      //  Repository member一覧
    @Published var selectedMemberIDs: Set<UUID>     //  選択中member ID一覧
    @Published var comment = ""                     //  通知コメント入力値

    init(repository: Repository) {
        self.repository = repository
        self.members = repository.members                           //  初期member一覧
        self.selectedMemberIDs = Set(repository.members.map(\.id))  //  初期状態では全memberを選択
        self.adminMember = repository.members.first                 //  先頭memberを管理者として利用
    }

    //  選択中member一覧
    var selectedMembers: [RepositoryMember] {
        members.filter { selectedMemberIDs.contains($0.id) }
    }

    //  通知送信可能か
    var canSend: Bool {
        selectedMembers.isEmpty == false
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

     //  Mock member追加
    func addMockMember() {
        //  次のmember番号
        let nextNumber = members.count + 1
        let member = RepositoryMember(login: "member\(nextNumber)")
        //  member一覧へ追加
        members.append(member)
        //  追加時は自動選択
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
}

