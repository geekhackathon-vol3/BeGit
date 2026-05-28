//  MemberAvatarRowView.swift
//  Team memberのavatar一覧表示

import SwiftUI

//  member avatar横並び表示View
struct MemberAvatarRowView: View {
    let members: [RepositoryMember] //  表示対象member一覧
    var avatarSize: CGFloat = 38    //  avatarサイズ
    var visibleLimit = 6            //  表示するavatar最大数

    var body: some View {
        //  avatar重なり表示
        HStack(spacing: -10) {
            //  表示上限数までavatar表示
            ForEach(Array(members.prefix(visibleLimit))) { member in
                AvatarView(member: member, size: avatarSize)
                    //  avatar境界線
                    .overlay(
                        Circle()
                            .stroke(AppTheme.background, lineWidth: 2)
                    )
            }

            //  表示しきれないmember数表示
            if members.count > visibleLimit {
                Text("+\(members.count - visibleLimit)")
                    .font(.system(size: 12, weight: .black, design: .monospaced))
                    .foregroundStyle(AppTheme.accent)
                    //  avatarサイズに合わせる
                    .frame(width: avatarSize, height: avatarSize)
                    .background(Color.white.opacity(0.08))  //  overflow avatar背景
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

