//  RepositoryActivityCardView.swift
//  Repository DashboardとResultで表示するTimeline card

import SwiftUI
import UIKit

//  Repository Timeline activity一覧
struct RepositoryActivityTimelineView: View {
    let activities: [RepositoryActivity]    //  表示対象activity一覧

    //  Timeline日付見出しFormatter
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

                //  activity背景画像同士をつなぐtimeline線
                if index < activities.count - 1 {
                    Rectangle()
                        .fill(Color(red: 0.333, green: 0.345, blue: 0.365))
                        .frame(width: 6, height: 18)
                        .padding(.leading, 43)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
    }

    //  日付が変わるactivityの前に見出しを表示する
    private func shouldShowDateHeader(at index: Int) -> Bool {
        guard index > 0 else {
            return true
        }

        return Calendar.current.isDate(
            activities[index].date,
            inSameDayAs: activities[index - 1].date
        ) == false
    }

    //  Timeline日付見出し
    private func dateHeader(for date: Date) -> some View {
        Text(Self.dayFormatter.string(from: date))
            .font(.system(size: 13, weight: .black, design: .monospaced))
            .foregroundStyle(.white.opacity(0.50))
            .textCase(.uppercase)
            .padding(.top, 4)
            .padding(.bottom, 10)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

//  Repository Timeline activity card
struct RepositoryActivityCardView: View {
    let activity: RepositoryActivity    //  表示対象activity
    @State private var isLiked: Bool    //  いいねON/OFF状態

    init(activity: RepositoryActivity) {
        self.activity = activity
        _isLiked = State(initialValue: false)
    }

    //  activity日時表示Formatter
    private static let dateFormatter: DateFormatter = {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "ja_JP")
        formatter.dateFormat = "M月d日 HH:mm"   //  日時表示形式
        return formatter
    }()

    var body: some View {
        //  activity card本体
        ZStack(alignment: .topLeading) {
            //  activity画像をcard全面の背景として表示
            activityBackground

            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .top) {
                    //  BeReal風の縦長thumbnail枠
                    activityThumbnailFrame
                }

                Spacer()

                //  activityタイトル
                Text(activity.title)
                    .font(.system(size: 17, weight: .black, design: .monospaced))
                    .foregroundStyle(.white)
                    .multilineTextAlignment(.center)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .center)

                Spacer()

                HStack(alignment: .center, spacing: 10) {
                    //  activity実行member avatar
                    AvatarView(member: activity.author, size: 34)
                        .background(
                            Circle()
                                .fill(AppTheme.background.opacity(0.82))
                        )
                        .overlay(
                            Circle()
                                .stroke(Color.white.opacity(0.72), lineWidth: 1.5)
                        )

                    VStack(alignment: .leading, spacing: 3) {
                        //  GitHub login名
                        Text(activity.author.login)
                            .font(.system(size: 13, weight: .black, design: .monospaced))
                            .foregroundStyle(.white)

                        //  activity日時
                        Text(Self.dateFormatter.string(from: activity.date))
                            .font(.system(size: 11, weight: .semibold, design: .monospaced))
                            .foregroundStyle(.white.opacity(0.64))
                    }

                    Spacer()

                    //  いいねbutton
                    likeButton
                }
            }
            .padding(16)
            .zIndex(1)

            //  activity種別badge
            typeBadge
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
                .zIndex(2)
        }
        .frame(maxWidth: .infinity, minHeight: 248, alignment: .topLeading)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .stroke(Color.white.opacity(0.10), lineWidth: 1)
        )
    }

    // MARK: - Components

    //  activity背景画像表示
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
                    //  画像がないactivityの背景icon
                    Image(systemName: activity.imageName ?? activity.type.systemImage)
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

    //  BeReal風の小さな縦長thumbnail枠
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

     //  activity種別badge
    private var typeBadge: some View {
        HStack(spacing: 5) {
            Image(activity.type.badgeIconName)
                .resizable()
                .scaledToFit()
                .frame(width: 17, height: 17)

            Text(activity.type.badgeTitle)
                .font(.system(size: 13, weight: .black, design: .monospaced))
                .foregroundStyle(.black)
        }
            .padding(.horizontal, 12)
            .padding(.vertical, 9)
            .background(activity.type.tint)
            .clipShape(BottomLeadingRoundedRectangle(cornerRadius: 10))
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

    //  いいねON/OFF button
    private var likeButton: some View {
        Button {
            isLiked.toggle()
        } label: {
            Image(systemName: isLiked ? "heart.fill" : "heart")
                .font(.system(size: 15, weight: .black))
                .foregroundStyle(isLiked ? AppTheme.softPink : .white.opacity(0.72))
                .frame(width: 34, height: 34)
                .background(Color.black.opacity(isLiked ? 0.42 : 0.30))
        }
        .buttonStyle(.plain)
            .clipShape(Circle())
    }
}

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
        case .sorry:
            "sorry"
        }
    }

    //  activity badge icon
    var badgeIconName: String {
        switch self {
        case .commit:
            "begit_badge_commit"
        case .pullRequest:
            "begit_badge_pr"
        case .sorry:
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
