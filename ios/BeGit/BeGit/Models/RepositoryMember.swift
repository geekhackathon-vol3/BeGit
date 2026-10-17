//  RepositoryMember.swift
//  Repositoryに参加しているGitHubユーザーを表すモデル

import Foundation

enum RepositoryRole: String, Equatable, Hashable, Sendable {
    case owner
    case poster
    case viewer

    init(backendValue: String?) {
        self = backendValue == "member" ? .poster : RepositoryRole(rawValue: backendValue ?? "") ?? .viewer
    }

    var displayName: String {
        switch self {
        case .owner: "オーナー"
        case .poster: "投稿可能"
        case .viewer: "表示専用"
        }
    }
}

//  Repository member情報
struct RepositoryMember: Identifiable, Equatable, Hashable, Sendable {
    let id: UUID        //  member識別子
    let backendUserID: Int64?   //  Backend user ID
    let login: String   //  GitHub login名
    let avatarURL: URL? //  GitHub avatar画像URL
    let role: RepositoryRole

    init(
        id: UUID = UUID(),
        backendUserID: Int64? = nil,
        login: String,
        avatarURL: URL? = nil,
        role: RepositoryRole = .poster
    ) {
        self.id = id
        self.backendUserID = backendUserID
        self.login = login
        self.avatarURL = avatarURL
        self.role = role
    }
}
