//  RepositoryActivity.swift
//  Repository Dashboardに表示するTimeline activityモデル

import Foundation

//  Repository Timeline activity
struct RepositoryActivity: Identifiable, Equatable, Hashable, Sendable {
    let id: UUID                            //  activity識別子
    let type: RepositoryActivityType        //  activity種別
    let title: String                       //  activityタイトル
    let date: Date                          //  activity作成日時
    let imageName: String?                  //  activity画像名
    let author: RepositoryMember            //  activity実行ユーザー
    let reactions: [ActivityReaction]       //  リアクション一覧

    init(
        id: UUID = UUID(),
        type: RepositoryActivityType,
        title: String,
        date: Date = Date(),
        imageName: String? = nil,
        author: RepositoryMember,
        reactions: [ActivityReaction] = []
    ) {
        self.id = id
        self.type = type
        self.title = title
        self.date = date
        self.imageName = imageName
        self.author = author
        self.reactions = reactions
    }
}

//  Repository activity種別
enum RepositoryActivityType: String, CaseIterable, Hashable, Sendable {
    case commit         //  commit activity
    case pullRequest    //  Pull Request activity
    case memo           //  進捗メモ投稿
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
                imageName: "begit_timeline_mock",
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
                imageName: "begit_timeline_mock",
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
                imageName: "begit_timeline_mock",
                author: palm,
                reactions: [
                    ActivityReaction(type: .thumbsup, count: 2, reactedByMe: true),
                ]
            ),
            RepositoryActivity(
                type: .memo,
                title: "ビルド落としてた…直しました🙏",
                date: calendar.date(byAdding: .hour, value: -24, to: Date()) ?? Date(),
                imageName: "begit_timeline_mock",
                author: liruly,
                reactions: []
            ),
        ]
    }
}
