//  GitHubRepository.swift
//  GitHub APIから取得するRepository候補

import Foundation

//  GitHub上のRepository選択候補
struct GitHubRepository: Identifiable, Equatable, Sendable {
    let id: Int                 //  GitHub上のRepository ID
    let fullName: String        //  owner/repository形式のRepository名
    let description: String?    //  Repository説明文
    let isPrivate: Bool         //  Private Repositoryかどうか
    let ownerAvatarURL: URL?    //  Repository ownerのavatar画像URL
    let updatedAt: Date?        //  Repositoryの最終更新日時
    let isReadOnly: Bool        //  GitHub App未接続の公開リポジトリ

    init(
        id: Int,
        fullName: String,
        description: String?,
        isPrivate: Bool,
        ownerAvatarURL: URL?,
        updatedAt: Date?,
        isReadOnly: Bool = false
    ) {
        self.id = id
        self.fullName = fullName
        self.description = description
        self.isPrivate = isPrivate
        self.ownerAvatarURL = ownerAvatarURL
        self.updatedAt = updatedAt
        self.isReadOnly = isReadOnly
    }
}
