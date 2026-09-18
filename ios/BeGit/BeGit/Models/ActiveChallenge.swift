//  ActiveChallenge.swift
//  グループで進行中の BeGit Time!（1時間チャレンジ）。GET /groups/:id の active_challenge に対応する。
//  無い（nil）= 進行中のチャレンジなし。

import Foundation

struct ActiveChallenge: Hashable, Sendable {
    //  発行者。グループを離脱済みの場合 login は空文字
    struct Issuer: Hashable, Sendable {
        let userID: Int64
        let login: String
        let avatarURL: URL?
    }

    //  自分（ログインユーザー）のこのチャレンジへの投稿
    struct MyPost: Hashable, Sendable {
        let postID: Int64
        let isDraft: Bool       //  true = ② Nice Work! の下書き（撮影して確定する対象）
        let status: String?     //  "on_time" | "late"
    }

    let notificationID: Int64
    let sprintID: Int64
    let issuer: Issuer
    let sentAt: Date
    let endsAt: Date            //  締め切り（発行 + 1h）
    let canEnd: Bool            //  自分が発行者で途中終了できるか
    let myPost: MyPost?

    //  残り時間（秒）。締め切り到達後は 0
    func remainingSeconds(at now: Date = Date()) -> TimeInterval {
        max(endsAt.timeIntervalSince(now), 0)
    }

    //  自分の Nice Work! 下書きがあり、アプリ内から撮影に進めるか
    var hasDraftToCapture: Bool {
        myPost?.isDraft == true
    }

    //  投稿済み（下書きではない）
    var hasPosted: Bool {
        guard let myPost else { return false }
        return myPost.isDraft == false
    }
}
