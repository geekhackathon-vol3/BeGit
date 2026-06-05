//  MemberSelectionRowView.swift
//  通知作成画面のmember選択行

import SwiftUI

struct MemberSelectionRowView: View {
    let member: RepositoryMember
    let isSelected: Bool
    let isAdmin: Bool
    let onToggle: () -> Void
    let onRemove: () -> Void

    var body: some View {
        HStack(spacing: 12) {
            Button(action: onToggle) {
                Image(systemName: isSelected ? "checkmark.circle.fill" : "circle")
                    .font(.system(size: 22, weight: .bold))
                    .foregroundStyle(isSelected ? AppTheme.accent : AppTheme.Text.muted)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("\(member.login)の通知対象")
            .accessibilityValue(isSelected ? "選択済み" : "未選択")
            .accessibilityHint("ダブルタップで選択状態を切り替えます")

            AvatarView(member: member, size: 38)

            VStack(alignment: .leading, spacing: 4) {
                HStack(spacing: 8) {
                    Text(member.login)
                        .appFont(.subheadline)
                        .foregroundStyle(AppTheme.Text.primary)
                        .lineLimit(1)

                    if isAdmin {
                        Text("ADMIN")
                            .appFont(.small)
                            .foregroundStyle(.black)
                            .padding(.horizontal, 7)
                            .padding(.vertical, 4)
                            .background(AppTheme.softPink)
                            .clipShape(Capsule())
                    }
                }

                Text(isSelected ? "notification target" : "muted locally")
                    .appFont(.caption)
                    .foregroundStyle(AppTheme.Text.disabled)
            }

            Spacer()

            Button(action: onRemove) {
                Image(systemName: "minus")
                    .font(.system(size: 12, weight: .black))
                    .foregroundStyle(AppTheme.softPink)
                    .frame(width: 32, height: 32)
                    .background(AppTheme.fieldBackground)
                    .clipShape(Circle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("\(member.login)を削除")
        }
        .padding(14)
        .background(Color.white.opacity(0.06))
        .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 20, style: .continuous)
                .stroke(isSelected ? AppTheme.accent.opacity(0.32) : Color.white.opacity(0.08), lineWidth: 1)
        )
    }
}
