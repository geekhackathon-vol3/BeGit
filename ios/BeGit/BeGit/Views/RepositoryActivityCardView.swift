//  RepositoryActivityCardView.swift
//  Repository DashboardとResultで表示するTimeline card

import SwiftUI

//  Repository Timeline activity一覧
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
                        .fill(Color(red: 0.333, green: 0.345, blue: 0.365))
                        .frame(width: 6, height: 30)
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
            .font(.system(size: 13, weight: .black, design: .monospaced))
            .foregroundStyle(.white.opacity(0.50))
            .textCase(.uppercase)
            .padding(.leading, 4)
            .padding(.bottom, 6)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

//  Repository Timeline activity card
struct RepositoryActivityCardView: View {
    let activity: RepositoryActivity
    @State private var showReactionPicker = false
    @State private var myReaction: ActivityReactionType?
    @State private var reactionCounts: [ActivityReactionType: Int]
    @State private var isSwapped = false //  背景と小窓の写真を入れ替えているか
    @State private var thumbnailScale: CGFloat = 1.0 //  小窓タップ時の弾みアニメ

    init(activity: RepositoryActivity) {
        self.activity = activity
        _myReaction = State(initialValue: activity.reactions.first(where: { $0.reactedByMe })?.type)
        var counts: [ActivityReactionType: Int] = [:]
        for r in activity.reactions { counts[r.type] = r.count }
        _reactionCounts = State(initialValue: counts)
    }

    //  背面/前面の両方の写真がある時だけ入れ替え可能
    private var canSwap: Bool {
        activity.mainPhotoURL != nil && activity.frontPhotoURL != nil
    }

    //  入れ替え状態を反映した表示用URL
    private var displayedMainURL: URL? {
        isSwapped ? activity.frontPhotoURL : activity.mainPhotoURL
    }

    private var displayedFrontURL: URL? {
        isSwapped ? activity.mainPhotoURL : activity.frontPhotoURL
    }

    //  activity日時表示Formatter
    private static let dateFormatter: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "ja_JP")
        f.dateFormat = "M月d日 HH:mm"
        return f
    }()

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            //  著者・日時（カード背景の外側・上部）
            authorHeader
                .padding(.bottom, 6)

            //  card本体 + リアクションピッカー
            ZStack(alignment: .bottomTrailing) {
                cardContent

                if showReactionPicker {
                    reactionPicker
                        .padding(.trailing, 16)
                        .padding(.bottom, reactionPickerBottomOffset)
                        .transition(.scale(scale: 0.6, anchor: .bottomTrailing).combined(with: .opacity))
                }
            }
            .animation(.spring(response: 0.28, dampingFraction: 0.68), value: showReactionPicker)
        }
    }

    // MARK: - Card

    private var cardContent: some View {
        ZStack(alignment: .topLeading) {
            //  背景画像
            activityBackground
                .onTapGesture {
                    guard showReactionPicker else { return }
                    withAnimation(.spring(response: 0.25, dampingFraction: 0.7)) {
                        showReactionPicker = false
                    }
                }

            //  投稿テキスト：背景画像全体の中央に絶対配置。
            //  コメントがあればコメントを表示（commit名は出さない）。無ければcommit名。
            Group {
                if let comment = activity.comment, comment.isEmpty == false {
                    Text(comment)
                        .font(.system(size: 15, weight: .semibold, design: .monospaced))
                        .foregroundStyle(.white.opacity(0.92))
                } else {
                    Text(activity.title)
                        .font(.system(size: 17, weight: .black, design: .monospaced))
                        .foregroundStyle(.white)
                }
            }
            .multilineTextAlignment(.center)
            .fixedSize(horizontal: false, vertical: true)
            .padding(.horizontal, 24)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)
            .zIndex(1)

            //  サムネ（左上）・リアクション（右下）
            VStack(alignment: .leading, spacing: 0) {
                HStack(alignment: .top) {
                    //  小窓タップで背景と入れ替え（何度でも可）＋ぽよよんアニメ
                    activityThumbnailFrame
                        .scaleEffect(thumbnailScale)
                        .contentShape(Rectangle())
                        .onTapGesture {
                            guard canSwap else { return }
                            //  背景⇄小窓をバネで入れ替え
                            withAnimation(.spring(response: 0.4, dampingFraction: 0.7)) {
                                isSwapped.toggle()
                            }
                            //  ぽよよん：素早く拡大しきってから、よく弾むバネで戻す
                            withAnimation(.easeOut(duration: 0.07)) {
                                thumbnailScale = 1.25
                            } completion: {
                                withAnimation(.spring(response: 0.28, dampingFraction: 0.3)) {
                                    thumbnailScale = 1.0
                                }
                            }
                        }
                        .sensoryFeedback(.impact(weight: .light), trigger: isSwapped)
                }
                Spacer()

                HStack(alignment: .center) {
                    if !displayedReactions.isEmpty {
                        reactionCountsRow
                    } else {
                        Spacer()
                    }
                    reactionButton
                }
            }
            .padding(16)
            .zIndex(2)

            //  activity種別badge（右上）
            typeBadge
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topTrailing)
                .zIndex(3)
        }
        .frame(maxWidth: .infinity)
        .aspectRatio(3/4, contentMode: .fit)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .stroke(Color.white.opacity(0.10), lineWidth: 1)
        )
    }

    // MARK: - Author header（カード外・上部）

    private var authorHeader: some View {
        HStack(alignment: .center, spacing: 10) {
            AvatarView(member: activity.author, size: 34)
                .padding(.leading, 4)
                .background(Circle().fill(AppTheme.background.opacity(0.82)))
                .overlay(Circle().stroke(Color.white.opacity(0.72), lineWidth: 1.5))

            VStack(alignment: .leading, spacing: 3) {
                Text(activity.author.login)
                    .font(.system(size: 13, weight: .black, design: .monospaced))
                    .foregroundStyle(.white)
                Text(Self.dateFormatter.string(from: activity.date))
                    .font(.system(size: 11, weight: .semibold, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.64))
            }

            Spacer()
        }
    }

    // MARK: - Reaction picker

    private var reactionPicker: some View {
        HStack(spacing: 2) {
            ForEach(ActivityReactionType.allCases, id: \.self) { type in
                Button {
                    toggleReaction(type)
                } label: {
                    Text(type.emoji)
                        .font(.system(size: 22))
                        .frame(width: 44, height: 44)
                        .background(
                            Circle()
                                .fill(myReaction == type
                                      ? Color.white.opacity(0.25)
                                      : Color.clear)
                        )
                        .scaleEffect(myReaction == type ? 1.15 : 1.0)
                        .animation(.spring(response: 0.22, dampingFraction: 0.6), value: myReaction)
                }
                .buttonStyle(.plain)
            }
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 6)
        .background(
            Capsule()
                .fill(Color.black.opacity(0.78))
                .overlay(Capsule().stroke(Color.white.opacity(0.14), lineWidth: 1))
        )
        .shadow(color: .black.opacity(0.35), radius: 12, x: 0, y: 4)
    }

    //  ピッカーがreactionButtonの上に来るよう下端からのオフセットを計算
    private var reactionPickerBottomOffset: CGFloat {
        58   // 16 padding + 34 button + 8 gap
    }

    // MARK: - Reaction button

    private var reactionButton: some View {
        Button {
            withAnimation(.spring(response: 0.3, dampingFraction: 0.65)) {
                showReactionPicker.toggle()
            }
        } label: {
            Group {
                if let r = myReaction {
                    Text(r.emoji)
                        .font(.system(size: 20))
                } else {
                    Image(systemName: "face.smiling")
                        .font(.system(size: 15, weight: .bold))
                        .foregroundStyle(.white.opacity(0.72))
                }
            }
            .frame(width: 34, height: 34)
        }
        .buttonStyle(.plain)
    }

    // MARK: - Reaction counts

    private var reactionCountsRow: some View {
        HStack(spacing: 6) {
            ForEach(displayedReactions, id: \.type) { reaction in
                HStack(spacing: 3) {
                    Text(reaction.type.emoji)
                        .font(.system(size: 13))
                    Text("\(reaction.count)")
                        .font(.system(size: 11, weight: .bold, design: .monospaced))
                        .foregroundStyle(.white.opacity(0.85))
                }
                .padding(.horizontal, 7)
                .padding(.vertical, 3)
                .background(
                    Capsule()
                        .fill(reaction.reactedByMe
                              ? Color.white.opacity(0.20)
                              : Color.white.opacity(0.08))
                )
            }
            Spacer()
        }
    }

    private var displayedReactions: [ActivityReaction] {
        ActivityReactionType.allCases
            .compactMap { type -> ActivityReaction? in
                let count = reactionCounts[type, default: 0]
                guard count > 0 else { return nil }
                return ActivityReaction(type: type, count: count, reactedByMe: myReaction == type)
            }
            .sorted { $0.count > $1.count }
    }

    // MARK: - Toggle logic

    private func toggleReaction(_ type: ActivityReactionType) {
        withAnimation(.spring(response: 0.25, dampingFraction: 0.65)) {
            if myReaction == type {
                reactionCounts[type, default: 1] -= 1
                if reactionCounts[type, default: 0] <= 0 { reactionCounts.removeValue(forKey: type) }
                myReaction = nil
            } else {
                if let prev = myReaction {
                    reactionCounts[prev, default: 1] -= 1
                    if reactionCounts[prev, default: 0] <= 0 { reactionCounts.removeValue(forKey: prev) }
                }
                reactionCounts[type, default: 0] += 1
                myReaction = type
            }
            showReactionPicker = false
        }
    }

    // MARK: - Subviews

    private var activityBackground: some View {
        GeometryReader { proxy in
            ZStack {
                cardBackground

                if let mainPhotoURL = displayedMainURL {
                    //  背面写真（実写真）を背景に表示
                    AsyncImage(url: mainPhotoURL) { phase in
                        switch phase {
                        case .success(let image):
                            image
                                .resizable()
                                .scaledToFill()
                        case .empty:
                            ProgressView()
                                .tint(.white)
                        case .failure:
                            Image(systemName: activity.type.systemImage)
                                .font(.system(size: 86, weight: .black))
                                .foregroundStyle(activity.type.tint.opacity(0.30))
                        @unknown default:
                            Color.clear
                        }
                    }
                    .frame(width: proxy.size.width, height: proxy.size.height)
                    .clipped()
                } else if let imageName = activity.imageName, UIImage(named: imageName) != nil {
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
                    colors: [.black.opacity(0.24), .black.opacity(0.30), .black.opacity(0.82)],
                    startPoint: .top,
                    endPoint: .bottom
                )

                LinearGradient(
                    colors: [.black.opacity(0.24), .black.opacity(0.02)],
                    startPoint: .leading,
                    endPoint: .trailing
                )
            }
            .frame(width: proxy.size.width, height: proxy.size.height)
        }
    }

    //  BeReal風の小さな縦長thumbnail枠（前面写真）
    private var activityThumbnailFrame: some View {
        ZStack {
            if let frontPhotoURL = displayedFrontURL {
                //  前面写真（セルフィー）を小窓に表示
                AsyncImage(url: frontPhotoURL) { phase in
                    switch phase {
                    case .success(let image):
                        image
                            .resizable()
                            .scaledToFill()
                    case .empty:
                        activity.type.tint.opacity(0.16)
                        ProgressView()
                            .tint(.white)
                    case .failure:
                        thumbnailFallback
                    @unknown default:
                        Color.clear
                    }
                }
            } else {
                thumbnailFallback
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

    //  前面写真が無い場合の小窓フォールバック
    private var thumbnailFallback: some View {
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
        .frame(width: 120, height: 160)
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
                .font(.system(size: 13, weight: .black, design: .monospaced))
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
                Color(red: 0.11, green: 0.08, blue: 0.14),
                AppTheme.cardBackground
            ],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
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
        case .commit:      "commit"
        case .pullRequest: "PR"
        case .memo:        "sorry"
        }
    }

    var badgeIconName: String {
        switch self {
        case .commit:      "begit_badge_commit"
        case .pullRequest: "begit_badge_pr"
        case .memo:        "begit_badge_sorry"
        }
    }

    var tint: Color {
        switch self {
        case .commit:      Color(red: 0.45, green: 0.94, blue: 0.67)
        case .pullRequest: Color(red: 1.00, green: 0.47, blue: 0.65)
        case .memo:        Color(red: 0.47, green: 0.74, blue: 1.00)
        }
    }

    var systemImage: String {
        switch self {
        case .commit:      "checkmark.seal"
        case .pullRequest: "arrow.triangle.pull"
        case .memo:        "hand.raised"
        }
    }
}
