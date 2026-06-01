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
}
