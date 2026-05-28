//  MemberChipView.swift
//  Repositoryメンバーを表示する小さなチップUI

import SwiftUI

struct MemberChipView: View {
    let member: RepositoryMember    //  表示対象のRepository member
    var onRemove: (() -> Void)?     //  member削除時のcallback

    var body: some View {
        //  member chip本体
        HStack(spacing: 8) {
            //  member avatar
            AvatarView(member: member, size: 22)

            //  GitHub login名
            Text(member.login)
                .font(.system(size: 13, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white.opacity(0.88))
                .lineLimit(1)

            //  削除ボタン表示
            if let onRemove {
                Button(action: onRemove) {
                     //  member削除アイコン
                    Image(systemName: "xmark")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundStyle(AppTheme.accent)
                        .frame(width: 18, height: 18)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("\(member.login)を削除")
            }
        }
        //  chip padding
        .padding(.vertical, 7)
        .padding(.leading, 8)
        //  削除ボタン有無で右padding調整
        .padding(.trailing, onRemove == nil ? 12 : 8)
        //  chip背景
        .background(Color.white.opacity(0.08))
        //  Capsule shape
        .clipShape(Capsule())
        //  chip border
        .overlay(
            Capsule()
                .stroke(Color.white.opacity(0.10), lineWidth: 1)
        )
    }
}

