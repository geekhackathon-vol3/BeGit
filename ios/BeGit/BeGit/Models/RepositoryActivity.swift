//  RepositoryActivity.swift
//  Repository Dashboardに表示するTimeline activityモデル

import Foundation

//  Repository Timeline activity
struct RepositoryActivity: Identifiable, Equatable, Hashable, Sendable {

    enum ID: Hashable, Sendable {
        case backendPost(Int64)
        case local(UUID)
    }

    let id: ID                          //  activity識別子（Backend投稿は再取得しても不変）
    let backendPostID: Int64?           //  Backend上の投稿ID（削除・リアクションに使用）
    let notificationID: Int64?          //  投稿が属するBeGit Time通知
    
    let type: RepositoryActivityType    //  activity種別
    let title: String                   //  activityタイトル
    let comment: String?                //  投稿時に入力したコメント（body）
    let date: Date                      //  activity作成日時
    let imageName: String?              //  背景画像名（Mock/Asset 用）
    let frontImageName: String?         //  左上小窓画像名（Mock/Asset 用）
    let mainPhotoURL: URL?              //  背面写真の presigned URL（背景表示用）
    let frontPhotoURL: URL?             //  前面写真の presigned URL（BeReal 小窓表示用）
    let author: RepositoryMember        //  activity実行ユーザー
    var reactions: [ActivityReaction]   //  リアクション一覧

    init(
        id: ID? = nil,
        backendPostID: Int64? = nil,
        notificationID: Int64? = nil,
        type: RepositoryActivityType,
        title: String,
        comment: String? = nil,
        date: Date = Date(),
        imageName: String? = nil,
        frontImageName: String? = nil,
        mainPhotoURL: URL? = nil,
        frontPhotoURL: URL? = nil,
        author: RepositoryMember,
        reactions: [ActivityReaction] = []
    ) {
        self.id = id ?? backendPostID.map(ID.backendPost) ?? .local(UUID())
        self.backendPostID = backendPostID
        self.notificationID = notificationID
        self.type = type
        self.title = title
        self.comment = comment
        self.date = date
        self.imageName = imageName
        self.frontImageName = frontImageName
        self.mainPhotoURL = mainPhotoURL
        self.frontPhotoURL = frontPhotoURL
        self.author = author
        self.reactions = reactions
    }

    // 実写真またはPreview用画像を持つ投稿だけ、写真カードとして表示する。
    // GitHub活動のみの実投稿に、写真投稿の見た目を誤って付けないために使用する。
    var hasVisualMedia: Bool {
        mainPhotoURL != nil || frontPhotoURL != nil || imageName != nil || frontImageName != nil
    }

    // 定期取得では同じ投稿のView identity・リアクション・署名付き画像URLを維持する。
    // R2の署名クエリだけが変わった場合にAsyncImageを再読み込みさせない。
    func preservingClientState(from current: RepositoryActivity) -> RepositoryActivity {
        guard backendPostID != nil, backendPostID == current.backendPostID else { return self }

        return RepositoryActivity(
            id: current.id,
            backendPostID: backendPostID,
            notificationID: notificationID,
            type: type,
            title: title,
            comment: comment,
            date: date,
            imageName: imageName,
            frontImageName: frontImageName,
            mainPhotoURL: Self.preferredPhotoURL(current: current.mainPhotoURL, refreshed: mainPhotoURL),
            frontPhotoURL: Self.preferredPhotoURL(current: current.frontPhotoURL, refreshed: frontPhotoURL),
            author: author,
            reactions: current.reactions
        )
    }

    static func mergingRefresh(
        _ refreshed: [RepositoryActivity],
        preserving current: [RepositoryActivity]
    ) -> [RepositoryActivity] {
        let currentByPostID = current.reduce(into: [Int64: RepositoryActivity]()) { result, activity in
            guard let postID = activity.backendPostID else { return }
            result[postID] = activity
        }

        return refreshed.map { activity in
            guard let postID = activity.backendPostID,
                  let existing = currentByPostID[postID] else { return activity }
            return activity.preservingClientState(from: existing)
        }
    }

    private static func preferredPhotoURL(current: URL?, refreshed: URL?) -> URL? {
        guard let refreshed else { return nil }
        guard let current else { return refreshed }

        let currentResource = (current.scheme, current.host, current.path)
        let refreshedResource = (refreshed.scheme, refreshed.host, refreshed.path)
        return currentResource == refreshedResource ? current : refreshed
    }
}

//  Repository activity種別
enum RepositoryActivityType: String, CaseIterable, Hashable, Sendable {
    case commit = "commit"                  //  commit activity
    case pullRequest = "pull_request"       //  Pull Request activity
    case memo = "memo"                      //  進捗メモ投稿

    var displayName: String {
        switch self {
        case .commit: "commit"
        case .pullRequest: "PR"
        case .memo: "進捗"
        }
    }

    static func fromAPIValue(_ value: String?) -> RepositoryActivityType {
        let normalized = value?
            .trimmingCharacters(in: .whitespacesAndNewlines)
            .lowercased()

        switch normalized {
        case "pull_request", "pullrequest", "pull-request", "pr", "review": return .pullRequest
        case "memo", "sorry", "comment", "issue": return .memo
        default: return .commit
        }
    }
}

//  リアクション種別（バックエンドと一致）
enum ActivityReactionType: String, CaseIterable, Hashable, Sendable {
    case heart
    case thumbsup
    case celebrate
    case fire
    case rocket

    var emoji: String {
        switch self {
        case .heart:     "❤️"
        case .thumbsup:  "👍"
        case .celebrate: "🎉"
        case .fire:      "🔥"
        case .rocket:    "🚀"
        }
    }
}

//  activity単体リアクション（種別 + 件数 + 自分がリアクション済みか）
struct ActivityReaction: Hashable, Sendable {
    let type: ActivityReactionType
    var count: Int
    var reactedByMe: Bool
}

//  Preview / Mock表示用activity
extension RepositoryActivity {
    static func mockActivities(for repository: Repository) -> [RepositoryActivity] {
        let riochin  = RepositoryMember(login: "Riochin",      avatarURL: URL(string: "https://avatars.githubusercontent.com/u/175614867?v=4"))
        let tomoka   = RepositoryMember(login: "s2108tomoka",  avatarURL: URL(string: "https://avatars.githubusercontent.com/u/163800046?v=4"))
        let palm     = RepositoryMember(login: "palm7710",     avatarURL: URL(string: "https://avatars.githubusercontent.com/u/168710387?v=4"))
        let liruly   = RepositoryMember(login: "liruly",       avatarURL: URL(string: "https://avatars.githubusercontent.com/u/141731612?v=4"))
        let calendar = Calendar.current

        return [
            RepositoryActivity(
                type: .commit,
                title: "feat: タイムライン画面をリアルタイム対応",
                date: calendar.date(byAdding: .minute, value: -18, to: Date()) ?? Date(),
                imageName: "begit_mock_post_1",
                frontImageName: "begit_mock_front_1",
                author: riochin,
                reactions: [
                    ActivityReaction(type: .fire,   count: 3, reactedByMe: false),
                    ActivityReaction(type: .rocket, count: 1, reactedByMe: true),
                ]
            ),
            RepositoryActivity(
                type: .pullRequest,
                title: "fix: PR #42 ダッシュボードの認証フロー修正",
                date: calendar.date(byAdding: .hour, value: -3, to: Date()) ?? Date(),
                imageName: "begit_mock_post_2",
                frontImageName: "begit_mock_front_2",
                author: tomoka,
                reactions: [
                    ActivityReaction(type: .heart,     count: 2, reactedByMe: false),
                    ActivityReaction(type: .celebrate, count: 1, reactedByMe: false),
                ]
            ),
            RepositoryActivity(
                type: .commit,
                title: "refactor: リアクション機能をコンポーネント化",
                date: calendar.date(byAdding: .hour, value: -8, to: Date()) ?? Date(),
                imageName: "begit_mock_post_3",
                frontImageName: "begit_mock_front_3",
                author: palm,
                reactions: [
                    ActivityReaction(type: .thumbsup, count: 2, reactedByMe: true),
                ]
            ),
            RepositoryActivity(
                type: .memo,
                title: "ビルド落としてた…直しました🙏",
                date: calendar.date(byAdding: .hour, value: -24, to: Date()) ?? Date(),
                imageName: "begit_mock_post_4",
                frontImageName: "begit_mock_front_4",
                author: liruly,
                reactions: []
            ),
        ]
    }
}
