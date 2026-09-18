//  GitHubActivitySelection.swift
//  写真確認画面で投稿に紐づける GitHub データ

import Foundation

enum PostContentSource: String, CaseIterable, Sendable {
    case github
    case manual

    var displayName: String {
        switch self {
        case .github: "GitHubデータ"
        case .manual: "コメントのみ"
        }
    }
}

struct GitHubCommitSelection: Identifiable, Hashable, Sendable {
    var id: String { sha }

    let sha: String
    let message: String
    let authorLogin: String
    let date: String
    let additions: Int
    let deletions: Int
}

struct GitHubPullRequestSelection: Identifiable, Hashable, Sendable {
    var id: Int { number }

    let number: Int
    let title: String
    let authorLogin: String
    let state: String
    let merged: Bool
    let updatedAt: String
}
