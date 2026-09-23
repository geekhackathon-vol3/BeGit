//  RepositoryActivityCardView.swift
//  Repository DashboardとResultで表示するTimeline card

import SwiftUI

struct RepositoryActivityTimelineView: View {
    let activities: [RepositoryActivity]
    let currentUserLogin: String?
    let lockedPostRoute: RepositoryNavigationRoute?
    let activeNotificationID: Int64?
    let onDeleteRequested: ((RepositoryActivity) -> Void)?
    let onReactionTapped: ((UUID, ActivityReactionType) async throws -> [ActivityReaction]?)?

    init(
        activities: [RepositoryActivity],
        currentUserLogin: String? = nil,
        lockedPostRoute: RepositoryNavigationRoute? = nil,
        activeNotificationID: Int64? = nil,
        onDeleteRequested: ((RepositoryActivity) -> Void)? = nil,
        onReactionTapped: ((UUID, ActivityReactionType) async throws -> [ActivityReaction]?)? = nil
    ) {
        self.activities = activities
        self.currentUserLogin = currentUserLogin
        self.lockedPostRoute = lockedPostRoute
        self.activeNotificationID = activeNotificationID
        self.onDeleteRequested = onDeleteRequested
        self.onReactionTapped = onReactionTapped
    }

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

                RepositoryActivityCardView(
                    activity: activity,
                    lockedPostRoute: activity.notificationID == activeNotificationID ? lockedPostRoute : nil,
                    canDelete: canDelete(activity),
                    onDelete: {
                        onDeleteRequested?(activity)
                    },
                    onReactionTapped: onReactionTapped.map { action in
                        { type in try await action(activity.id, type) }
                    }
                )

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

    private func canDelete(_ activity: RepositoryActivity) -> Bool {
        guard let currentUserLogin,
              activity.backendPostID != nil else { return false }
        return activity.author.login.caseInsensitiveCompare(currentUserLogin) == .orderedSame
    }

    private func dateHeader(for date: Date) -> some View {
        Text(Self.dayFormatter.string(from: date))
            .appFont(.label)
            .foregroundStyle(AppTheme.Text.low)
            .textCase(.uppercase)
            .padding(.leading, 4)
            .padding(.bottom, 6)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

struct RepositoryActivityCardView: View {
    let activity: RepositoryActivity
    let lockedPostRoute: RepositoryNavigationRoute?
    let canDelete: Bool
    let onDelete: (() -> Void)?
    var onReactionTapped: ((ActivityReactionType) async throws -> [ActivityReaction]?)? = nil

    @State private var showReactionPicker = false
    @State private var myReaction: ActivityReactionType?
    @State private var reactionCounts: [ActivityReactionType: Int]
    @State private var isUpdatingReaction = false
    @State private var showReactionError = false
    @State private var isSwapped = false //  背景と小窓の写真を入れ替えているか
    @State private var thumbnailScale: CGFloat = 1.0 //  小窓タップ時の弾みアニメ

    init(
        activity: RepositoryActivity,
        lockedPostRoute: RepositoryNavigationRoute? = nil,
        canDelete: Bool = false,
        onDelete: (() -> Void)? = nil,
        onReactionTapped: ((ActivityReactionType) async throws -> [ActivityReaction]?)? = nil
    ) {
        self.activity = activity
        self.lockedPostRoute = lockedPostRoute
        self.canDelete = canDelete
        self.onDelete = onDelete
        self.onReactionTapped = onReactionTapped
        _myReaction = State(initialValue: activity.reactions.first(where: { $0.reactedByMe })?.type)
        var counts: [ActivityReactionType: Int] = [:]
        for r in activity.reactions { counts[r.type] = r.count }
        _reactionCounts = State(initialValue: counts)
    }

    //  実写真（URL）とモック画像（アセット名）のどちらでも、
    //  背面/前面の両方がある時だけ入れ替え可能。
    private var canSwap: Bool {
        let hasMainPhoto = activity.mainPhotoURL != nil || activity.imageName != nil
        let hasFrontPhoto = activity.frontPhotoURL != nil || activity.frontImageName != nil
        return hasMainPhoto && hasFrontPhoto
    }

    //  入れ替え状態を反映した表示用URL
    private var displayedMainURL: URL? {
        isSwapped ? activity.frontPhotoURL : activity.mainPhotoURL
    }

    private var displayedFrontURL: URL? {
        isSwapped ? activity.mainPhotoURL : activity.frontPhotoURL
    }

    //  モック画像用の表示名も、入れ替え状態に合わせて反転する。
    private var displayedMainImageName: String? {
        isSwapped ? activity.frontImageName : activity.imageName
    }

    private var displayedFrontImageName: String? {
        isSwapped ? activity.imageName : activity.frontImageName
    }

    private static let timeFormatter: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "ja_JP")
        f.dateFormat = "HH:mm"
        return f
    }()

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            //  著者・日時（カード背景の外側・上部）
            authorHeader
                .padding(.bottom, 12)

            //  card本体（写真 + 投稿テキスト）
            cardContent
        }
    }

    //  コメントがあればコメントを表示（commit名は出さない）。無ければcommit名。
    private var postText: some View {
        Group {
            if let comment = activity.comment, comment.isEmpty == false {
                Text(comment)
                    .font(.system(size: 14, weight: .regular, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.92))
            } else {
                Text(activity.title)
                    .font(.system(size: 14, weight: .regular, design: .monospaced))
                    .foregroundStyle(.white)
            }
        }
        .multilineTextAlignment(.leading)
        .fixedSize(horizontal: false, vertical: true)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    // MARK: - Card

    //  「写真」と「その下の投稿テキスト」を縦に並べる。
    //  テキストは写真に重ねず、写真の明るさに左右されず読めるようにする。
    private var cardContent: some View {
        Group {
            if activity.isLocked {
                lockedContent
            } else {
                unlockedCardContent
            }
        }
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
    }

    private var unlockedCardContent: some View {
        VStack(alignment: .leading, spacing: 0) {
            //  写真 + リアクションピッカー（ピッカーは写真の右下基準で出す）
            ZStack(alignment: .bottomTrailing) {
                photoArea

                if showReactionPicker {
                    reactionPicker
                        .padding(.trailing, 16)
                        .padding(.bottom, reactionPickerBottomOffset)
                        .transition(.scale(scale: 0.6, anchor: .bottomTrailing).combined(with: .opacity))
                        .zIndex(10)
                }
            }
            .animation(.spring(response: 0.28, dampingFraction: 0.68), value: showReactionPicker)

            postText
                .padding(.horizontal, 16)
                .padding(.top, 12)
                .padding(.bottom, 20)
        }
        .onChange(of: activity.reactions) { _, reactions in
            applyReactions(reactions)
        }
        .alert("リアクションの更新に失敗しました", isPresented: $showReactionError) {
            Button("OK", role: .cancel) {}
        }
    }

    private var lockedContent: some View {
        ZStack {
            LinearGradient(
                colors: [
                    Color(red: 0.05, green: 0.04, blue: 0.07),
                    Color(red: 0.28, green: 0.25, blue: 0.30),
                    Color(red: 0.26, green: 0.15, blue: 0.12)
                ],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )

            Circle()
                .fill(Color.white.opacity(0.18))
                .frame(width: 260, height: 260)
                .blur(radius: 70)
                .offset(x: -90, y: -40)

            Circle()
                .fill(activity.type.tint.opacity(0.20))
                .frame(width: 240, height: 240)
                .blur(radius: 78)
                .offset(x: 105, y: 220)

            VStack(spacing: 18) {
                Image(systemName: "eye.slash.fill")
                    .font(.system(size: 42, weight: .semibold))

                Text("投稿して表示")
                    .font(.system(size: 25, weight: .bold))

                Text("あなたの進捗をシェアして\nメンバーの投稿を見てみましょう。")
                    .font(.system(size: 16, weight: .regular))
                    .multilineTextAlignment(.center)
                    .lineSpacing(4)
                    .foregroundStyle(.white.opacity(0.88))

                if let lockedPostRoute {
                    NavigationLink(value: lockedPostRoute) {
                        Text("進捗を投稿する")
                            .font(.system(size: 17, weight: .bold))
                            .foregroundStyle(.black)
                            .padding(.horizontal, 28)
                            .frame(height: 50)
                            .background(.white)
                            .clipShape(Capsule())
                    }
                    .buttonStyle(.plain)
                    .padding(.top, 4)
                }
            }
            .foregroundStyle(.white)
            .padding(28)
        }
        .frame(maxWidth: .infinity)
        .aspectRatio(3/4, contentMode: .fit)
        .accessibilityElement(children: .combine)
        .accessibilityLabel("投稿内容は非表示です。自分の進捗を投稿すると表示されます")
    }

    private var photoArea: some View {
        ZStack(alignment: .topLeading) {
            //  背景画像
            activityBackground
                .allowsHitTesting(false)

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
        .clipped()
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
                Text(Self.timeFormatter.string(from: activity.date))
                    .font(.system(size: 11, weight: .semibold, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.64))
            }

            Spacer()

            if canDelete {
                Menu {
                    Button(role: .destructive) {
                        onDelete?()
                    } label: {
                        Label {
                            Text("削除")
                                .foregroundStyle(.red)
                        } icon: {
                            Image(systemName: "trash")
                                .foregroundStyle(.red)
                        }
                        .tint(.red)
                    }
                } label: {
                    Image(systemName: "ellipsis")
                        .font(.system(size: 20, weight: .semibold))
                        .foregroundStyle(AppTheme.Text.high)
                        .frame(width: 36, height: 36)
                        .contentShape(Rectangle())
                }
                .accessibilityLabel("投稿メニュー")
            }
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
                .disabled(isUpdatingReaction)
                .contentShape(Circle())
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
        68   // 16 padding + 44 button + 8 gap
    }

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
            .frame(width: 44, height: 44)
            .background(Circle().fill(Color.black.opacity(0.42)))
            .contentShape(Circle())
        }
        .buttonStyle(.plain)
        .disabled(isUpdatingReaction)
        .contentShape(Circle())
        .zIndex(4)
        .accessibilityLabel(showReactionPicker ? "スタンプを閉じる" : "スタンプを選ぶ")
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
        guard isUpdatingReaction == false else { return }

        guard let onReactionTapped else {
            applyLocalToggle(type)
            return
        }

        showReactionPicker = false
        isUpdatingReaction = true
        let isRemovingSelectedReaction = myReaction == type
        Task {
            defer { isUpdatingReaction = false }
            do {
                if let reactions = try await onReactionTapped(type) {
                    applyReactions(
                        reactions,
                        preferredMyReaction: isRemovingSelectedReaction ? nil : type
                    )
                } else {
                    // APIの対象IDを持たないMock投稿は従来どおりローカル更新する。
                    applyLocalToggle(type)
                }
            } catch {
                showReactionError = true
            }
        }
    }

    private func applyLocalToggle(_ type: ActivityReactionType) {
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

    private func applyReactions(
        _ reactions: [ActivityReaction],
        preferredMyReaction: ActivityReactionType? = nil
    ) {
        withAnimation(.spring(response: 0.25, dampingFraction: 0.65)) {
            var counts: [ActivityReactionType: Int] = [:]
            for reaction in reactions {
                counts[reaction.type] = reaction.count
            }
            reactionCounts = counts
            myReaction = preferredMyReaction ?? reactions.first(where: \.reactedByMe)?.type
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
                } else if let imageName = displayedMainImageName, UIImage(named: imageName) != nil {
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
        .frame(width: 72, height: 96)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(Color.black, lineWidth: 2)
        )
        .shadow(color: .black.opacity(0.32), radius: 10, x: 0, y: 5)
    }

    //  前面写真が無い場合の小窓フォールバック（モック時は表示中のアセットを使用）
    private var thumbnailFallback: some View {
        ZStack {
            if let name = displayedFrontImageName, UIImage(named: name) != nil {
                Image(name)
                    .resizable()
                    .scaledToFill()
            } else if UIImage(named: "begit_github_character") != nil {
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
    }

    private var typeBadge: some View {
        HStack(spacing: 5) {
            if activity.type == .memo {
                Image(systemName: activity.type.systemImage)
                    .font(.system(size: 15, weight: .bold))
                    .frame(width: 17, height: 17)
            } else {
                Image(activity.type.badgeIconName)
                    .resizable()
                    .scaledToFit()
                    .frame(width: 17, height: 17)
            }

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
        displayName
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
        case .memo:        "pencil.and.list.clipboard"
        }
    }
}
