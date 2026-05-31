//  RepositoryActivity.swift
//  Repository Dashboardに表示するTimeline activityモデル

import Foundation

//  Repository Timeline activity
struct RepositoryActivity: Identifiable, Equatable, Hashable, Sendable {
    let id: UUID                        //  activity識別子
    let type: RepositoryActivityType    //  activity種別
    let title: String                   //  activityタイトル
    let comment: String?                //  activity補足コメント
    let date: Date                      //  activity作成日時
    let imageName: String?              //  activity画像名
    let author: RepositoryMember        //  activity実行ユーザー
    let reaction: RepositoryReaction?   //  activityリアクション

    init(
        id: UUID = UUID(),
        type: RepositoryActivityType,
        title: String,
        comment: String? = nil,
        date: Date = Date(),
        imageName: String? = nil,
        author: RepositoryMember,
        reaction: RepositoryReaction? = nil
    ) {
        self.id = id
        self.type = type
        self.title = title
        self.comment = comment
        self.date = date
        self.imageName = imageName
        self.author = author
        self.reaction = reaction
    }
}

//  Repository activity種別
enum RepositoryActivityType: String, CaseIterable, Hashable, Sendable {
    case commit         //  commit activity
    case pullRequest    //  Pull Request activity
    case sorry          //  謝罪・障害報告activity
}

//  activityリアクション種別
enum RepositoryReaction: String, Hashable, Sendable {
    case heart  //  お気に入りリアクション
    case check  //  完了リアクション
    case sorry  //  謝罪リアクション
}

//  Preview / Mock表示用activity
extension RepositoryActivity {
    static func mockActivities(for repository: Repository) -> [RepositoryActivity] {
        //  member未設定時のFallback member
        let members = repository.members.isEmpty
            ? [RepositoryMember(login: "begit")]
            : repository.members

        return [
            //  commit activity mock
            RepositoryActivity(
                type: .commit,
                title: "Implemented realtime repository home",
                comment: "UI polish and state wiring landed before the notification window.",
                date: Date(timeIntervalSinceNow: -60 * 18),
                imageName: "begit_timeline_mock",
                author: members[0],
                reaction: .check
            ),
            //  Pull Request activity mock
            RepositoryActivity(
                type: .pullRequest,
                title: "Opened PR for dashboard flow",
                comment: "Needs review on navigation and activity card density.",
                date: Date(timeIntervalSinceNow: -60 * 64),
                imageName: "begit_timeline_mock",
                author: members[min(1, members.count - 1)],
                reaction: .heart
            ),
            //  障害・謝罪activity mock
            RepositoryActivity(
                type: .sorry,
                title: "Sorry, build was red for 12 minutes",
                comment: "Fixed the missing Combine import and re-ran simulator build.",
                date: Date(timeIntervalSinceNow: -60 * 130),
                imageName: "begit_timeline_mock",
                author: members[min(2, members.count - 1)],
                reaction: .sorry
            )
        ]
    }
}
