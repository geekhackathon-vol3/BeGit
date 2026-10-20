//  DraftPost.swift
//  写真添付の対象となる本人の報告（確定済みも含む）

import Foundation

struct DraftPost: Hashable, Sendable {
    let id: Int64
    let repoFullName: String    //  下書き作成時に検知したリポジトリ（owner/repo）
    let postType: RepositoryActivityType //  GitHubイベントから自動決定された投稿種別
    let status: String?         //  "on_time" | "late"
}
