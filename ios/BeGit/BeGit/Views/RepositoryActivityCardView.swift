//  RepositoryActivityCardView.swift
//  Repository DashboardとResultで表示するTimeline card

import SwiftUI
import UIKit

//  Repository Timeline activity card
struct RepositoryActivityCardView: View {
    let activity: RepositoryActivity    //  表示対象activity

    //  activity日時表示Formatter
    private static let dateFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.dateFormat = "MMM d, HH:mm"   //  日時表示形式
        return formatter
    }()

    var body: some View {
        //  activity card本体
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top) {
                //  activity image表示
                activityImage

                Spacer(minLength: 12)

                //  activity種別badge
                typeBadge
            }

            HStack(alignment: .center, spacing: 10) {
                //  activity実行member avatar
                AvatarView(member: activity.author, size: 34)

                VStack(alignment: .leading, spacing: 3) {
                    //  GitHub login名
                    Text(activity.author.login)
                        .font(.system(size: 13, weight: .black, design: .monospaced))
                        .foregroundStyle(.white)

                    //  activity日時
                    Text(Self.dateFormatter.string(from: activity.date))
                        .font(.system(size: 11, weight: .semibold, design: .monospaced))
                        .foregroundStyle(.white.opacity(0.42))
                }

                Spacer()

                //  activityリアクション表示
                if let reaction = activity.reaction {
                    reactionIcon(for: reaction)
                }
            }

            //  activityタイトル
            Text(activity.title)
                .font(.system(size: 17, weight: .black, design: .monospaced))
                .foregroundStyle(.white)
                .fixedSize(horizontal: false, vertical: true)

            //  activityコメント表示
            if let comment = activity.comment, comment.isEmpty == false {
                Text(comment)
                    .font(.system(size: 13, weight: .medium, design: .monospaced))
                    .foregroundStyle(AppTheme.softPink.opacity(0.82))
                    .lineSpacing(4)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(16)
        .background(cardBackground)
        .clipShape(RoundedRectangle(cornerRadius: 24, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 24, style: .continuous)
                .stroke(Color.white.opacity(0.10), lineWidth: 1)
        )
    }

    // MARK: - Components

    //  activity画像表示
    private var activityImage: some View {
        ZStack {
            //  activity背景色
            RoundedRectangle(cornerRadius: 20, style: .continuous)
                .fill(activity.type.tint.opacity(0.14))

            //  activity画像
            if let imageName = activity.imageName, UIImage(named: imageName) != nil {
                Image(imageName)
                    .resizable()
                    .scaledToFill()
            } else {
                //  activity icon
                Image(systemName: activity.imageName ?? activity.type.systemImage)
                    .font(.system(size: 30, weight: .black))
                    .foregroundStyle(activity.type.tint)
            }
        }
        .frame(height: 132)
        .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
    }

     //  activity種別badge
    private var typeBadge: some View {
        Text(activity.type.badgeTitle)
            .font(.system(size: 11, weight: .black, design: .monospaced))
            .foregroundStyle(.black)
            .padding(.horizontal, 10)
            .padding(.vertical, 7)
            .background(activity.type.tint)
            .clipShape(Capsule())
    }

    //  card背景Gradient
    private var cardBackground: some View {
        LinearGradient(
            colors: [
                Color(red: 0.11, green: 0.08, blue: 0.14),
                AppTheme.cardBackground
            ],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
    }

     //  reaction icon表示
    private func reactionIcon(for reaction: RepositoryReaction) -> some View {
        Image(systemName: reaction.systemImage)
            .font(.system(size: 14, weight: .black))
            .foregroundStyle(reaction.tint)
            .frame(width: 34, height: 34)
            .background(reaction.tint.opacity(0.14))
            .clipShape(Circle())
    }
}

//  activity種別UI定義
private extension RepositoryActivityType {
    var badgeTitle: String {
        switch self {
        case .commit:
            "commit"
        case .pullRequest:
            "PR"
        case .sorry:
            "sorry"
        }
    }

    //  activityテーマカラー
    var tint: Color {
        switch self {
        case .commit:
            Color(red: 0.45, green: 0.94, blue: 0.67)
        case .pullRequest:
            Color(red: 1.00, green: 0.47, blue: 0.65)
        case .sorry:
            Color(red: 0.47, green: 0.74, blue: 1.00)
        }
    }

    //  activity icon
    var systemImage: String {
        switch self {
        case .commit:
            "checkmark.seal"
        case .pullRequest:
            "arrow.triangle.pull"
        case .sorry:
            "hand.raised"
        }
    }
}

//  reaction UI定義
private extension RepositoryReaction {
    //  reaction icon
    var systemImage: String {
        switch self {
        case .heart:
            "heart.fill"
        case .check:
            "checkmark.circle.fill"
        case .sorry:
            "bubble.left.and.exclamationmark.bubble.right.fill"
        }
    }

    //  reactionテーマカラー
    var tint: Color {
        switch self {
        case .heart:
            AppTheme.softPink
        case .check:
            Color(red: 0.45, green: 0.94, blue: 0.67)
        case .sorry:
            Color(red: 0.47, green: 0.74, blue: 1.00)
        }
    }
}
