//  MemberAvatarRowView.swift
//  Team memberのavatar一覧表示

import SwiftUI

//  member avatar横並び表示View
struct MemberAvatarRowView: View {
    let members: [RepositoryMember]             //  表示対象member一覧
    var avatarSize: CGFloat = 38                //  avatarサイズ
    var visibleLimit = 6                        //  表示するavatar最大数
    var avatarSpacing: CGFloat = -10            //  avatar間隔
    var achievedMemberIDs: Set<UUID> = []       //  投稿達成済みmember ID一覧
    var achievedMemberLogins: Set<String> = []   //  投稿達成済みmember login一覧
    var dimUnachieved = false                   //  未達成memberを暗く表示するか

    var body: some View {
        //  avatar重なり表示
        HStack(spacing: avatarSpacing) {
            //  表示上限数までavatar表示
            ForEach(Array(members.prefix(visibleLimit))) { member in
                ZStack(alignment: .bottomTrailing) {
                    AvatarView(member: member, size: avatarSize)
                        //  avatar境界線
                        .overlay(
                            Circle()
                                .stroke(AppTheme.background, lineWidth: 2)
                        )

                    if dimUnachieved && !isAchieved(member) {
                        Circle()
                            .fill(Color.black.opacity(0.52))
                            .frame(width: avatarSize, height: avatarSize)
                    }

                    //  投稿達成済みcheck mark
                    if isAchieved(member) {
                        Image("begit_check_mark")
                            .resizable()
                            .scaledToFit()
                            .frame(width: avatarSize * 0.42, height: avatarSize * 0.42)
                            .offset(x: avatarSize * 0.06, y: avatarSize * 0.06)
                    }
                }
                .frame(width: avatarSize, height: avatarSize)
            }

            //  表示しきれないmember数表示
            if members.count > visibleLimit {
                Text("+\(members.count - visibleLimit)")
                    .appFont(.sectionHeader)
                    .foregroundStyle(AppTheme.accent)
                    //  avatarサイズに合わせる
                    .frame(width: avatarSize, height: avatarSize)
                    .background(AppTheme.fieldBackground)  //  overflow avatar背景
                    .clipShape(Circle())                    //  Circle shape
                    //  avatar境界線
                    .overlay(
                        Circle()
                            .stroke(AppTheme.background, lineWidth: 2)
                    )
            }
        }
    }

    private func isAchieved(_ member: RepositoryMember) -> Bool {
        achievedMemberIDs.contains(member.id) || achievedMemberLogins.contains(member.login)
    }
}

//  BeGit Timeの参加人数と達成状況をTimeline/Resultで共通表示するView
struct BeGitTimeProgressSummaryView: View {
    let members: [RepositoryMember]
    let achievedMemberLogins: Set<String>
    let dimUnachieved: Bool
    var comment: String? = nil

    private var completedCount: Int {
        members.filter { achievedMemberLogins.contains($0.login) }.count
    }

    private var progress: CGFloat {
        guard members.isEmpty == false else { return 0 }
        return CGFloat(completedCount) / CGFloat(members.count)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            MemberAvatarRowView(
                members: members,
                avatarSize: 42,
                achievedMemberLogins: achievedMemberLogins,
                dimUnachieved: dimUnachieved
            )

            if let comment, comment.isEmpty == false {
                Text(comment)
                    .appFont(.body)
                    .foregroundStyle(AppTheme.softPink.opacity(0.82))
                    .lineSpacing(4)
            }

            GeometryReader { proxy in
                ZStack(alignment: .leading) {
                    Capsule()
                        .fill(Color.white.opacity(0.12))

                    Capsule()
                        .fill(AppTheme.accent)
                        .frame(width: proxy.size.width * progress)

                    Text("\(completedCount)/\(members.count)人が参加しました！")
                        .appFont(.body)
                        .foregroundStyle(Color.black.opacity(0.76))
                        .lineLimit(1)
                        .minimumScaleFactor(0.78)
                        .frame(maxWidth: .infinity, alignment: .center)
                }
            }
            .frame(height: 30)
        }
    }
}
