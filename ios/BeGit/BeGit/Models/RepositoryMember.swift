//  RepositoryMember.swift
//  Repositoryに参加しているGitHubユーザーを表すモデル

import Foundation

//  Repository member情報
struct RepositoryMember: Identifiable, Equatable, Sendable {
    let id: UUID        //  member識別子
    let login: String   //  GitHub login名
    let avatarURL: URL? //  GitHub avatar画像URL

    init(id: UUID = UUID(), login: String, avatarURL: URL? = nil) {
        self.id = id
        self.login = login
        self.avatarURL = avatarURL
    }
}

