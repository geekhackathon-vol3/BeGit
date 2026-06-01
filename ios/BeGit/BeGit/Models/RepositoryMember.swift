//  RepositoryMember.swift
//  Repositoryに参加しているGitHubユーザーを表すモデル

import Foundation

//  Repository member情報
struct RepositoryMember: Identifiable, Equatable, Hashable, Sendable {
    let id: UUID        //  member識別子
    let backendUserID: Int64?   //  Backend user ID
    let login: String   //  GitHub login名
    let avatarURL: URL? //  GitHub avatar画像URL

    init(id: UUID = UUID(), backendUserID: Int64? = nil, login: String, avatarURL: URL? = nil) {
        self.id = id
        self.backendUserID = backendUserID
        self.login = login
        self.avatarURL = avatarURL
    }
}
