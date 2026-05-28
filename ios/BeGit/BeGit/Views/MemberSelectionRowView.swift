//  MemberSelectionRowView.swift
//  通知作成画面のmember選択行

import SwiftUI

//  通知対象memberを選択・削除する行View
struct MemberSelectionRowView: View {
    let member: RepositoryMember    //  表示対象member
    let isSelected: Bool            //  通知対象として選択されているか
    let isAdmin: Bool               //  管理者memberかどうか
    let onToggle: () -> Void        //  選択状態切り替え処理
    let onRemove: () -> Void        //  member削除処理

    var body: some View {
        //  member選択行本体
        HStack(spacing: 12) {
            //  選択状態切り替えボタン
            Button(action: onToggle) {
                Image(systemName: isSelected ? "checkmark.circle.fill" : "circle")
                    .font(.system(size: 22, weight: .bold))
                    .foregroundStyle(isSelected ? AppTheme.accent : Color.white.opacity(0.34))
            }
            .buttonStyle(.plain)

            //  member avatar
            AvatarView(member: member, size: 38)

            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 8) {
                    //  GitHub login名
                    Text(member.login)
                        .font(.system(size: 15, weight: .black, design: .monospaced))
                        .foregroundStyle(.white)
                        .lineLimit(1)

                    //  管理者badge
                    if isAdmin {
                        Text("ADMIN")
                            .font(.system(size: 9, weight: .black, design: .monospaced))
                            .foregroundStyle(.black)
                            .padding(.horizontal, 7)
                            .padding(.vertical, 4)
                            .background(AppTheme.softPink)
                            .clipShape(Capsule())
                    }
                }

                //  選択状態説明
                Text(isSelected ? "notification target" : "muted locally")
                    .font(.system(size: 11, weight: .semibold, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.42))
            }

            Spacer()

            //  member削除ボタン
            Button(action: onRemove) {
                Image(systemName: "minus")
                    .font(.system(size: 12, weight: .black))
                    .foregroundStyle(AppTheme.softPink)
                    .frame(width: 32, height: 32)
                    .background(Color.white.opacity(0.07))
                    .clipShape(Circle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("\(member.login)を削除")
        }
        .padding(14)                                                        //  row padding
        .background(Color.white.opacity(0.06))                              //  row背景
        .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))  //  row shape
        //  選択状態に応じてborder色を変更
        .overlay(
            RoundedRectangle(cornerRadius: 20, style: .continuous)
                .stroke(isSelected ? AppTheme.accent.opacity(0.32) : Color.white.opacity(0.08), lineWidth: 1)
        )
    }
}

