//  NotificationPayload.swift
//  FCM data メッセージ（通知7種）を型安全にパースするモデル。
//  契約は docs/notification/ios-guide.md §2。FCM の data は値がすべて文字列で届くため、
//  数値フィールドは Int 変換する。未知 type / 必須欠落は init? が nil を返し、安全に無視する。

import Foundation

//  通知の種類（FCM data の `type`）
enum NotificationType: String {
    case begitTime = "begit_time"           // ① BeGit Time!（チャレンジ開始）
    case niceWork = "nice_work"             // ② Nice Work!（自分の初アクティビティ検知）
    case challengeEnd = "challenge_end"     // ③ チャレンジ終了（結果サマリ）
    case sprintReminder = "sprint_reminder" // ④ スプリント終了3日前
    case sprintEnd = "sprint_end"           // ⑤ スプリント終了
    case sprintStart = "sprint_start"       // ⑥ 新スプリント開始
    case reaction                           // ⑦ リアクション
    case comment                            // ⑦ コメント
}

//  FCM data ペイロードをパースした結果。type 別に使うフィールドが異なる。
struct NotificationPayload: Equatable {
    let type: NotificationType
    let groupId: Int            // 共通: 対象グループ ID
    let notificationId: Int?    // ①③: anchor となった BeGit Time 通知
    let sprintId: Int?          // ④⑤⑥: 対象スプリント
    let draftPostId: Int?       // ②: プレフィル元の下書き post
    let postId: Int?            // ⑦: 反応された投稿
    let status: String?         // ②: "on_time" | "late"
    let actorLogin: String?     // ⑦: 反応した相手の login

    //  FCM の userInfo（didReceive 等で渡る辞書）からパースする。
    //  type が未知、または共通必須（type / group_id）が欠落・非数値なら nil。
    init?(userInfo: [AnyHashable: Any]) {
        guard let typeString = Self.string(userInfo["type"]),
              let type = NotificationType(rawValue: typeString),
              let groupIdString = Self.string(userInfo["group_id"]),
              let groupId = Int(groupIdString) else {
            return nil
        }

        self.type = type
        self.groupId = groupId
        self.notificationId = Self.int(userInfo["notification_id"])
        self.sprintId = Self.int(userInfo["sprint_id"])
        self.draftPostId = Self.int(userInfo["draft_post_id"])
        self.postId = Self.int(userInfo["post_id"])
        self.status = Self.string(userInfo["status"])
        self.actorLogin = Self.string(userInfo["actor_login"])
    }

    //  FCM data は文字列値だが、NSString 等で届く場合も吸収する。
    private static func string(_ value: Any?) -> String? {
        switch value {
        case let string as String:
            return string
        case let number as NSNumber:
            return number.stringValue
        default:
            return nil
        }
    }

    //  文字列で届く数値を Int に変換する。
    private static func int(_ value: Any?) -> Int? {
        guard let string = string(value) else { return nil }
        return Int(string)
    }
}

extension NotificationPayload {
    //  通知タップ時に遷移すべき画面 route へ変換する（#55 ではスタブ画面へ）。
    //  type 別フィールドが欠ける場合は遷移できないため nil。
    var route: RepositoryNavigationRoute? {
        switch type {
        case .begitTime:
            //  ① チャレンジ中なら memo 投稿 UI（中身は #56）
            return .notificationPostCreation(groupId: groupId, notificationId: notificationId)
        case .niceWork:
            //  ② 下書きプレフィル → 撮影 → 確定（中身は #56）
            guard let draftPostId else { return nil }
            return .notificationNiceWorkDraft(groupId: groupId, draftPostId: draftPostId, status: status)
        case .challengeEnd:
            //  ③ チャレンジ結果画面
            guard let notificationId else { return nil }
            return .notificationChallengeResult(groupId: groupId, notificationId: notificationId)
        case .sprintReminder, .sprintStart:
            //  ④⑥ スプリント概要画面
            guard let sprintId else { return nil }
            return .notificationSprintOverview(groupId: groupId, sprintId: sprintId)
        case .sprintEnd:
            //  ⑤ スプリント結果画面
            guard let sprintId else { return nil }
            return .notificationSprintResult(groupId: groupId, sprintId: sprintId)
        case .reaction, .comment:
            //  ⑦ 該当投稿の詳細（中身は #57）
            guard let postId else { return nil }
            let kind: NotificationSocialKind = (type == .reaction) ? .reaction : .comment
            return .notificationPostDetail(groupId: groupId, postId: postId, kind: kind)
        }
    }
}

//  ⑦ の遷移先で「リアクション一覧 / コメント一覧」どちらを開くかの区別。
enum NotificationSocialKind: Hashable, Sendable {
    case reaction
    case comment
}
