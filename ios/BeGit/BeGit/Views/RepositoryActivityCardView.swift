//  RepositoryActivityCardView.swift
//  Repository DashboardとResultで表示するTimeline card

import SwiftUI
import UIKit

struct RepositoryActivityTimelineView: View {
    let activities: [RepositoryActivity]

    private static let dayFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "ja_JP")
        formatter.dateFormat = "yyyy年M月d日"
        return formatter
    }()

    var body: some View {
        VStack(spacing: 0) {
            ForEach(Array(activities.enumerated()), id: \.element.id) { index, activity in
                if shouldShowDateHeader(at: index) {
                    dateHeader(for: activity.date)
                }

                RepositoryActivityCardView(activity: activity)

                if index < activities.count - 1 {
                    Rectangle()
                        .fill(AppTheme.borderSubtle)
                        .frame(width: 6, height: 18)
                        .padding(.leading, 43)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
    }

    private func shouldShowDateHeader(at index: Int) -> Bool {
        guard index > 0 else { return true }
        return Calendar.current.isDate(
            activities[index].date,
            inSameDayAs: activities[index - 1].date
        ) == false
    }

    private func dateHeader(for date: Date) -> some View {
        Text(Self.dayFormatter.string(from: date))
            .appFont(.label)
            .foregroundStyle(AppTheme.Text.low)
            .textCase(.uppercase)
            .padding(.top, 4)
            .padding(.bottom, 10)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

struct RepositoryActivityCardView: View {
    let activity: RepositoryActivity
    @State private var isLiked: Bool

    init(activity: RepositoryActivity) {
        self.activity = activity
        _isLiked = State(initialValue: false)
    }

    private static let dateFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "ja_JP")
        formatter.dateFormat = "M月d日 HH:mm"
        return formatter
    }()

    var body: some View {
        ZStack(alignment: .topLeading) {
            activityBackground

            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .top) {
                    activityThumbnailFrame
                }

                Spacer()

                Text(activity.title)
                    .appFont(.headline)                         // size:17,black → size:18,semibold で近似
                    .foregroundStyle(AppTheme.Text.primary)
                    .multilineTextAlignment(.center)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .center)

                Spacer()

                HStack(alignment: .center, spacing: 10) {
                    AvatarView(member: activity.author, size: 34)
                        .background(
                            Circle()
                                .fill(AppTheme.background.opacity(0.82))
                        )
                        .overlay(
                            Circle()
                                .stroke(AppTheme.Text.primary.opacity(0.72), lineWidth: 1.5)
                        )

                    VStack(alignment: .leading, spacing: 3) {
                        Text(activity.author.login)
                            .appFont(.label)
                            .foregroundStyle(AppTheme.Text.primary)

                        Text(Self.dateFormatter.string(from: activity.date))
                            .appFont(.caption)
                            .foregroundStyle(AppTheme.Text.medium)
                    }

                    Spacer()

                    likeButton
                }
            }
            .padding(16)
            .zIndex(1)

            typeBadge
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
                .zIndex(2)
        }
        .frame(maxWidth: .infinity, minHeight: 248, alignment: .topLeading)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .stroke(AppTheme.Text.primary.opacity(0.10), lineWidth: 1)
        )
    }

    // MARK: - Components

    private var activityBackground: some View {
        GeometryReader { proxy in
            ZStack {
                cardBackground

                if let imageName = activity.imageName, UIImage(named: imageName) != nil {
                    Image(imageName)
                        .resizable()
                        .scaledToFill()
                        .frame(width: proxy.size.width, height: proxy.size.height)
                        .clipped()
                } else {
                    Image(systemName: activity.type.systemImage)
                        .font(.system(size: 86, weight: .black))
                        .foregroundStyle(activity.type.tint.opacity(0.30))
                }

                LinearGradient(
                    colors: [
                        Color.black.opacity(0.24),
                        Color.black.opacity(0.30),
                        Color.black.opacity(0.82)
                    ],
                    startPoint: .top,
                    endPoint: .bottom
                )

                LinearGradient(
                    colors: [
                        Color.black.opacity(0.24),
                        Color.black.opacity(0.02)
                    ],
                    startPoint: .leading,
                    endPoint: .trailing
                )
            }
            .frame(width: proxy.size.width, height: proxy.size.height)
        }
    }

    private var activityThumbnailFrame: some View {
        ZStack {
            if UIImage(named: "begit_github_character") != nil {
                Image("begit_github_character")
                    .resizable()
                    .scaledToFill()
            } else {
                activity.type.tint.opacity(0.16)

                Image(systemName: activity.type.systemImage)
                    .font(.system(size: 22, weight: .black))
                    .foregroundStyle(activity.type.tint)
            }
        }
        .frame(width: 60, height: 80)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(Color.black, lineWidth: 2)
        )
        .shadow(color: .black.opacity(0.32), radius: 10, x: 0, y: 5)
    }

    private var typeBadge: some View {
        HStack(spacing: 5) {
            Image(activity.type.badgeIconName)
                .resizable()
                .scaledToFit()
                .frame(width: 17, height: 17)

            Text(activity.type.badgeTitle)
                .appFont(.label)
                .foregroundStyle(.black)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 9)
        .background(activity.type.tint)
        .clipShape(BottomLeadingRoundedRectangle(cornerRadius: 10))
    }

    private var cardBackground: some View {
        LinearGradient(
            colors: [
                Color(red: 0.11, green: 0.08, blue: 0.14),  // card固有色のため保持
                AppTheme.cardBackground
            ],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
    }

    private var likeButton: some View {
        Button {
            isLiked.toggle()
        } label: {
            Image(systemName: isLiked ? "heart.fill" : "heart")
                .font(.system(size: 15, weight: .black))
                .foregroundStyle(isLiked ? AppTheme.softPink : AppTheme.Text.high)
                .frame(width: 34, height: 34)
                .background(Color.black.opacity(isLiked ? 0.42 : 0.30))
        }
        .buttonStyle(.plain)
        .clipShape(Circle())
    }
}

// 以降は変更なし（BottomLeadingRoundedRectangle, RepositoryActivityType extension）

//  左下だけ丸角のbadge shape
private struct BottomLeadingRoundedRectangle: Shape {
    let cornerRadius: CGFloat

    func path(in rect: CGRect) -> Path {
        let radius = min(cornerRadius, rect.width / 2, rect.height / 2)

        var path = Path()
        path.move(to: CGPoint(x: rect.minX, y: rect.minY))
        path.addLine(to: CGPoint(x: rect.maxX, y: rect.minY))
        path.addLine(to: CGPoint(x: rect.maxX, y: rect.maxY))
        path.addLine(to: CGPoint(x: rect.minX + radius, y: rect.maxY))
        path.addQuadCurve(
            to: CGPoint(x: rect.minX, y: rect.maxY - radius),
            control: CGPoint(x: rect.minX, y: rect.maxY)
        )
        path.addLine(to: CGPoint(x: rect.minX, y: rect.minY))
        path.closeSubpath()

        return path
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
        case .memo:
            "memo"
        }
    }

    //  activity badge icon
    var badgeIconName: String {
        switch self {
        case .commit:
            "begit_badge_commit"
        case .pullRequest:
            "begit_badge_pr"
        case .memo:
            "begit_badge_sorry"
        }
    }

    //  activityテーマカラー
    var tint: Color {
        switch self {
        case .commit:
            Color(red: 0.45, green: 0.94, blue: 0.67)
        case .pullRequest:
            Color(red: 1.00, green: 0.47, blue: 0.65)
        case .memo:
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
        case .memo:
            "hand.raised"
        }
    }
}
