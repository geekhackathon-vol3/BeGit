//  AddRepositoryViewModel.swift
//  Repository追加画面の入力状態管理

import Foundation
import Combine

@MainActor
final class AddRepositoryViewModel: ObservableObject {
    @Published var repositoryURLText = ""                           //  Repository URL入力値
    @Published var memberLoginText = ""                             //  member login入力値
    @Published private(set) var members: [RepositoryMember] = []    //  追加済みmember一覧
    @Published var isMemberInputVisible = false                     //  member入力欄の表示状態

    //  Repository作成可能か
    var canComplete: Bool {
        repositoryName != nil
    }

    //  member追加可能か
    var canAddMember: Bool {
        normalizedMemberLogin.isEmpty == false
            && members.contains { $0.login.caseInsensitiveCompare(normalizedMemberLogin) == .orderedSame } == false
    }

    // MARK: - Actions

    //  member入力欄を表示
    func showMemberInput() {
        isMemberInputVisible = true
    }

    //  member追加
    func addMember() {
        guard canAddMember else { return }

        members.append(RepositoryMember(login: normalizedMemberLogin))
        memberLoginText = ""
        isMemberInputVisible = false
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
        let trimmedText = repositoryURLText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let url = URL(string: trimmedText), url.host?.lowercased() == "github.com" else {
            return nil
        }

        let pathComponents = url.pathComponents.filter { $0 != "/" }
        guard pathComponents.count >= 2 else { return nil }

        return "\(pathComponents[0])/\(pathComponents[1])"
    }
}
