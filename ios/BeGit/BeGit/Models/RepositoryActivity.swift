//  RepositoryActivity.swift
//  Repository Dashboardに表示するTimeline activityモデル

import Foundation

//  Repository Timeline activity
struct RepositoryActivity: Identifiable, Equatable, Hashable, Sendable {
    let id: UUID                        //  activity識別子
    let type: RepositoryActivityType    //  activity種別
    let title: String                   //  activityタイトル
    let date: Date                      //  activity作成日時
    let imageName: String?              //  activity画像名
    let author: RepositoryMember        //  activity実行ユーザー
    let reaction: RepositoryReaction?   //  activityリアクション

    init(
        id: UUID = UUID(),
        type: RepositoryActivityType,
        title: String,
        date: Date = Date(),
        imageName: String? = nil,
        author: RepositoryMember,
        reaction: RepositoryReaction? = nil
    ) {
        self.id = id
        self.type = type
        self.title = title
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
        let calendar = Calendar.current

        return [
            //  commit activity mock
            RepositoryActivity(
                type: .commit,
                title: "Implemented realtime repository home",
                date: calendar.date(byAdding: .minute, value: -18, to: Date()) ?? Date(),
                imageName: "begit_timeline_mock",
                author: members[0],
                reaction: .check
            ),
            //  Pull Request activity mock
            RepositoryActivity(
                type: .pullRequest,
                title: "Opened PR for dashboard flow",
                date: calendar.date(byAdding: .hour, value: -31, to: Date()) ?? Date(),
                imageName: "begit_timeline_mock",
                author: members[min(1, members.count - 1)],
                reaction: .heart
            ),
            //  障害・謝罪activity mock
            RepositoryActivity(
                type: .sorry,
                title: "Sorry, build was red for 12 minutes",
                date: calendar.date(byAdding: .hour, value: -76, to: Date()) ?? Date(),
                imageName: "begit_timeline_mock",
                author: members[min(2, members.count - 1)],
                reaction: .sorry
            )
        ]
    }
}
