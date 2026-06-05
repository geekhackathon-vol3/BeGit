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

                    //  投稿達成済みcheck mark
                    if achievedMemberIDs.contains(member.id) {
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
}
