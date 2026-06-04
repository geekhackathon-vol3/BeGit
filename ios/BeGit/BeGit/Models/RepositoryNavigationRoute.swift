//  RepositoryNavigationRoute.swift
//  Repository Home以降のpush遷移を表すroute

import Foundation

//  Repository画面遷移route
enum RepositoryNavigationRoute: Hashable, Sendable {
    //  Repository Dashboard画面
    case dashboard(Repository)
    //  カメラ画面
    case camera
    //  通知作成画面
    case makeNotification(Repository)
    //  通知結果画面
    case notificationResult(RepositoryNotification)

    // MARK: - FCM 通知タップからの deep link（#55 ルーティング基盤）
    // Push の data には ID しか載らないため、リッチな Repository / RepositoryNotification ではなく
    // ID ベースの route を持つ。遷移先 View の中身は後続 #56 / #57 が実装する（現状スタブ）。

    //  ① begit_time → memo 投稿 UI（#56）
    case notificationPostCreation(groupId: Int, notificationId: Int?)
    //  ② nice_work → 下書きプレフィル → 撮影 → 確定（#56）
    case notificationNiceWorkDraft(groupId: Int, draftPostId: Int, status: String?)
    //  ③ challenge_end → チャレンジ結果画面
    case notificationChallengeResult(groupId: Int, notificationId: Int)
    //  ④⑥ sprint_reminder / sprint_start → スプリント概要画面
    case notificationSprintOverview(groupId: Int, sprintId: Int)
    //  ⑤ sprint_end → スプリント結果画面
    case notificationSprintResult(groupId: Int, sprintId: Int)
    //  ⑦ reaction / comment → 投稿詳細（#57）
    case notificationPostDetail(groupId: Int, postId: Int, kind: NotificationSocialKind)
}
