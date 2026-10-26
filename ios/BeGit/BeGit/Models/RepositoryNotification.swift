//  RepositoryNotification.swift
//  BeReal風通知作成結果を表すローカルモデル

import Foundation

enum BeGitGameMode: String, Codable, CaseIterable, Hashable, Sendable {
    case everyone
    case spotlight

    var title: String {
        switch self {
        case .everyone: "みんな、今何作ってる？"
        case .spotlight: "俺の進捗を見ろ！"
        }
    }

    var detail: String {
        switch self {
        case .everyone: "選んだメンバー全員が時間内に報告すると成功"
        case .spotlight: "発行者だけが時間内に報告すると成功"
        }
    }

    var systemImage: String {
        self == .everyone ? "person.3.fill" : "crown.fill"
    }
}

enum BeGitGameResult: String, Codable, Hashable, Sendable {
    case pending
    case succeeded
    case failed
    case cancelled
}

//  Repository通知情報
struct RepositoryNotification: Identifiable, Equatable, Hashable, Sendable {
    let id: UUID                                //  通知識別子
    let backendID: Int64?                       //  Backend通知ID
    let repository: Repository                  //  対象Repository
    let selectedMembers: [RepositoryMember]     //  通知対象member一覧
    let comment: String                         //  通知コメント
    let createdAt: Date                         //  通知作成日時
    let gameMode: BeGitGameMode

    init(
        id: UUID = UUID(),
        backendID: Int64? = nil,
        repository: Repository,
        selectedMembers: [RepositoryMember],
        comment: String,
        createdAt: Date = Date(),
        gameMode: BeGitGameMode = .everyone
    ) {
        self.id = id
        self.backendID = backendID
        self.repository = repository
        self.selectedMembers = selectedMembers
        self.comment = comment
        self.createdAt = createdAt
        self.gameMode = gameMode
    }
}
