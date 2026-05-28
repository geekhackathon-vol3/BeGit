//  RepositoryNotification.swift
//  BeReal風通知作成結果を表すローカルモデル

import Foundation

//  Repository通知情報
struct RepositoryNotification: Identifiable, Equatable, Hashable, Sendable {
    let id: UUID                                //  通知識別子
    let repository: Repository                  //  対象Repository
    let selectedMembers: [RepositoryMember]     //  通知対象member一覧
    let comment: String                         //  通知コメント
    let createdAt: Date                         //  通知作成日時

    init(
        id: UUID = UUID(),
        repository: Repository,
        selectedMembers: [RepositoryMember],
        comment: String,
        createdAt: Date = Date()
    ) {
        self.id = id
        self.repository = repository
        self.selectedMembers = selectedMembers
        self.comment = comment
        self.createdAt = createdAt
    }
}

