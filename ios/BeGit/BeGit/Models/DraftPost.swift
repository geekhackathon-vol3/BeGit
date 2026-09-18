//  DraftPost.swift
//  ② Nice Work! でサーバーが作成した下書き投稿（撮影後に写真を付けて確定する対象）

import Foundation

struct DraftPost: Hashable, Sendable {
    let id: Int64
    let repoFullName: String    //  下書き作成時に検知したリポジトリ（owner/repo）
    let postType: RepositoryActivityType //  GitHubイベントから自動決定された投稿種別
    let status: String?         //  "on_time" | "late"
}
