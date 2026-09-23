//  RepositoryDashboardView.swift
//  Repository選択後のDashboard / Timeline画面

import SwiftUI

@MainActor
struct RepositoryDashboardView: View {
    @EnvironmentObject private var authState: AuthState
    //  Dashboard画面の状態を管理するViewModel
    @StateObject private var viewModel: RepositoryDashboardViewModel
    @State private var isShowingGallery = false
    @State private var isShowingRepoSetting = false
    @State private var activityToDelete: RepositoryActivity?
    @State private var isShowingDeleteConfirmation = false
    @State private var deleteErrorMessage = ""
    @State private var isShowingDeleteError = false
    @State private var isShowingStopConfirmation = false
    @State private var stopErrorMessage = ""
    @State private var isShowingStopError = false

    //  Dashboard画面の状態を管理するViewModel
    init(repository: Repository) {
        _viewModel = StateObject(wrappedValue: RepositoryDashboardViewModel(repository: repository))
    }

    //  外部ViewModel注入用
    init(viewModel: RepositoryDashboardViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        ZStack {
            AppTheme.background
                .ignoresSafeArea()
            dashboardContent
        }
        .animation(.easeInOut(duration: 0.20), value: isShowingGallery)
        .animation(.easeInOut(duration: 0.20), value: viewModel.activeChallenge)
        .navigationBarTitleDisplayMode(.inline)
        .navigationBarBackButtonHidden(true)
        .toolbar {
            ToolbarItem(placement: .topBarLeading) {
                BeGitBackButton()
            }

            ToolbarItem(placement: .principal) {
                BeGitToolbarLogoView()
            }

            ToolbarItem(placement: .topBarTrailing) {
                HStack(spacing: 0) {
                    Button {
                        isShowingGallery.toggle()
                    } label: {
                        Image(systemName: isShowingGallery ? "list.bullet" : "square.grid.3x3.fill")
                            .foregroundStyle(AppTheme.softPink)
                            .frame(width: 44, height: 44)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel(isShowingGallery ? "タイムラインに戻る" : "投稿写真一覧")

                    Button {
                        isShowingRepoSetting = true
                    } label: {
                        Image(systemName: "gearshape.fill")
                            .foregroundStyle(AppTheme.softPink)
                            .frame(width: 44, height: 44)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("リポジトリ設定")
                }
            }
        }
        .sheet(isPresented: $isShowingRepoSetting) {
            RepoSettingView(repository: viewModel.repository)
        }
        .confirmationDialog("この投稿を削除しますか？", isPresented: $isShowingDeleteConfirmation) {
            Button("削除する", role: .destructive) {
                guard let activity = activityToDelete else { return }
                activityToDelete = nil
                Task {
                    do {
                        try await viewModel.deleteActivity(
                            activity,
                            accessToken: authState.accessToken
                        )
                    } catch {
                        deleteErrorMessage = error.localizedDescription
                        isShowingDeleteError = true
                    }
                }
            }

            Button("キャンセル", role: .cancel) {
                activityToDelete = nil
            }
        } message: {
            Text("写真やコメントの表示だけが削除されます。GitHub上のcommitやPRは削除されません。")
        }
        .alert("削除できませんでした", isPresented: $isShowingDeleteError) {
            Button("OK", role: .cancel) { }
        } message: {
            Text(deleteErrorMessage)
        }
        .confirmationDialog("BeGit Timeを停止しますか？", isPresented: $isShowingStopConfirmation) {
            Button("Timeを停止", role: .destructive) {
                Task {
                    do {
                        try await viewModel.stopActiveBeGitTime(accessToken: authState.accessToken)
                    } catch {
                        stopErrorMessage = error.localizedDescription
                        isShowingStopError = true
                    }
                }
            }
            Button("キャンセル", role: .cancel) { }
        } message: {
            Text("停止すると、これ以降の投稿は受け付けません。すでに投稿された内容は残ります。")
        }
        .alert("BeGit Timeを停止できませんでした", isPresented: $isShowingStopError) {
            Button("OK", role: .cancel) { }
        } message: {
            Text(stopErrorMessage)
        }
        .toolbar(.hidden, for: .tabBar)
        .tint(AppTheme.accent)
        //  accessToken変更時に前のタスクを自動キャンセルしてリロード
        .task(id: authState.accessToken) {
            await viewModel.loadActivities(
                accessToken: authState.accessToken,
                currentUserID: authState.githubUser.map { Int64($0.id) }
            )
            while Task.isCancelled == false {
                try? await Task.sleep(for: .seconds(5))
                if Task.isCancelled { break }
                await viewModel.refreshActiveChallenge(accessToken: authState.accessToken)
            }
        }
        //  表示のたび（通知作成・撮影から戻ったときを含む）に進行中のBeGit Timeを取り直す
        .onAppear {
            Task { await viewModel.loadActiveChallenge(accessToken: authState.accessToken) }
        }
    }

    @ViewBuilder
    private var dashboardContent: some View {
        if isShowingGallery {
            RepositoryPhotoGalleryContentView(
                repository: viewModel.repository,
                activities: viewModel.activities
            )
            .transition(.opacity)
        } else {
            timelineScreen
        }
    }

    private var timelineScreen: some View {
        VStack(spacing: 0) {
            timelineScrollView
            timelinePostButton
        }
        .transition(.opacity)
    }

    private var timelineScrollView: some View {
        ScrollView {
            timelineStack
                .padding(.horizontal, 20)
                .padding(.top, 20)
                .padding(.bottom, 104)
        }
    }

    private var timelineStack: some View {
        LazyVStack(alignment: .leading, spacing: 18) {
            challengeStatusView
            timelineHeader

            if viewModel.isLoading {
                statusText("Loading timeline...")
            }

            if let errorMessage = viewModel.errorMessage {
                statusText(errorMessage)
            }

            BeGitTimeProgressSummaryView(
                members: viewModel.repository.members,
                achievedMemberLogins: achievedMemberLogins,
                dimUnachieved: shouldDimUnachievedMembers
            )
            activityTimeline
        }
    }

    @ViewBuilder
    private var challengeStatusView: some View {
        if let challenge = viewModel.activeChallenge {
            TimelineView(.periodic(from: .now, by: 1)) { context in
                if context.date < challenge.endsAt {
                    activeChallengeCard(
                        activeBeGitTime: ActiveBeGitTime(
                            notificationID: challenge.notificationID,
                            sentBy: challenge.issuer.userID,
                            sentAt: challenge.sentAt,
                            expiresAt: challenge.endsAt
                        ),
                        now: context.date,
                        canStop: challenge.canEnd,
                        draftPost: challenge.myPost,
                        onStop: { isShowingStopConfirmation = true }
                    )
                }
            }
        } else if let activeBeGitTime = viewModel.activeBeGitTime {
            //  active_challenge APIの反映待ちでも開催中表示は維持する。
            //  下書き情報が無いため、この状態では撮影導線を表示しない。
            TimelineView(.periodic(from: .now, by: 1)) { context in
                if context.date < activeBeGitTime.expiresAt {
                    activeChallengeCard(
                        activeBeGitTime: activeBeGitTime,
                        now: context.date,
                        canStop: isNotificationOwner(activeBeGitTime),
                        draftPost: nil,
                        onStop: { isShowingStopConfirmation = true }
                    )
                }
            }
        } else if viewModel.endedBeGitTime != nil {
            endedChallengeCard
        }
    }

    private var activityTimeline: some View {
        RepositoryActivityTimelineView(
            activities: viewModel.activities,
            currentUserLogin: authState.githubUser?.login,
            onDeleteRequested: {
                activityToDelete = $0
                isShowingDeleteConfirmation = true
            },
            onReactionTapped: { activityID, type in
                try await viewModel.toggleReaction(
                    activityID: activityID,
                    type: type,
                    accessToken: authState.accessToken,
                    currentUserID: authState.githubUser.map { Int64($0.id) }
                )
            }
        )
        .padding(.horizontal, -20)
    }

    @ViewBuilder
    private var timelinePostButton: some View {
        Group {
            if let challenge = viewModel.activeChallenge,
               challenge.hasDraftToCapture,
               let backendID = viewModel.repository.backendID,
               let draftPost = challenge.myPost {
                NavigationLink(
                    value: RepositoryNavigationRoute.notificationNiceWorkDraft(
                        groupId: Int(backendID),
                        draftPostId: Int(draftPost.postID),
                        status: draftPost.status
                    )
                ) {
                    PrimaryCapsuleButtonLabel(
                        title: "撮影して投稿",
                        systemImage: "camera.fill",
                        isEnabled: true
                    )
                }
            } else if viewModel.activeChallenge != nil || viewModel.activeBeGitTime != nil {
                PrimaryCapsuleButtonLabel(
                    title: "BeGit Time 進行中",
                    systemImage: "hourglass",
                    isEnabled: false
                )
                .accessibilityLabel("GitHubのcommitを待っています")
            } else {
                NavigationLink(value: RepositoryNavigationRoute.makeNotification(viewModel.repository)) {
                    PrimaryCapsuleButtonLabel(
                        title: "通知を作成する",
                        systemImage: "bolt.badge.clock",
                        isEnabled: true
                    )
                }
            }
        }
        .buttonStyle(.plain)
        .padding(.horizontal, 20)
        .padding(.top, 14)
        .padding(.bottom, 18)
        .background(bottomBarBackground)
    }

    // MARK: - Components

    //  Timeline画面Header
    private var timelineHeader: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Timeline")
                .font(.custom("Bitcount", size: 34))
                .foregroundStyle(.white)
                .frame(maxWidth: .infinity, alignment: .leading)

            Text(viewModel.repository.name)
                .font(.system(size: 12, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white.opacity(0.50))
                .lineLimit(1)
        }
    }

    private func activeChallengeCard(
        activeBeGitTime: ActiveBeGitTime,
        now: Date,
        canStop: Bool,
        draftPost: ActiveChallenge.MyPost?,
        onStop: @escaping () -> Void
    ) -> some View {
        let accentPurple = Color(red: 0.72, green: 0.58, blue: 0.98)
        let lightPurple = Color(red: 0.90, green: 0.84, blue: 1.00)
        let issuer = viewModel.repository.members.first(where: {
            $0.backendUserID == activeBeGitTime.sentBy
        }) ?? viewModel.repository.members.first
        let issuerLogin = issuer?.login ?? "メンバー"

        return VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .firstTextBaseline, spacing: 10) {
                Image(systemName: "hourglass")
                    .font(.system(size: 18, weight: .semibold))
                    .foregroundStyle(accentPurple)
                    .frame(width: 24)

                Text("BeGit Time")
                    .font(.system(size: 20, weight: .bold, design: .monospaced))
                    .foregroundStyle(.white)

                Text("開催中")
                    .font(.system(size: 13, weight: .bold))
                    .foregroundStyle(accentPurple)

                Spacer(minLength: 8)

                Text(remainingTimeText(until: activeBeGitTime.expiresAt, now: now))
                    .font(.system(size: 17, weight: .bold, design: .monospaced))
                    .foregroundStyle(AppTheme.softPink)
                    .monospacedDigit()
            }

            HStack(spacing: 10) {
                if let issuer {
                    AvatarView(member: issuer, size: 38)
                }

                VStack(alignment: .leading, spacing: 3) {
                    Text("\(issuerLogin) がスタート")
                        .font(.system(size: 16, weight: .semibold))
                        .foregroundStyle(.white)

                    Text("pushしたら投稿できるよ")
                        .font(.system(size: 14, weight: .medium))
                        .foregroundStyle(.white.opacity(0.68))
                }
            }

            HStack(spacing: 10) {
                if draftPost?.isDraft == true {
                    Text("Nice Work!が届きました。下のボタンから投稿できます")
                        .font(.system(size: 13, weight: .medium))
                        .foregroundStyle(AppTheme.accent)
                        .frame(maxWidth: .infinity, alignment: .leading)
                } else if draftPost?.isDraft == false {
                    HStack(spacing: 6) {
                        Image(systemName: draftPost?.status == "late" ? "clock.badge.exclamationmark" : "checkmark.seal.fill")
                        Text(draftPost?.status == "late" ? "投稿済み · Late" : "投稿済み · On Time")
                    }
                    .font(.system(size: 13, weight: .bold, design: .monospaced))
                    .foregroundStyle(draftPost?.status == "late" ? AppTheme.softPink : AppTheme.accent)
                } else {
                    Text("GitHubにcommitすると撮影して投稿できます")
                        .font(.system(size: 13, weight: .medium))
                        .foregroundStyle(.white.opacity(0.68))
                        .frame(maxWidth: .infinity, alignment: .leading)
                }

                if canStop {
                    Button("停止", action: onStop)
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(AppTheme.softPink)
                        .buttonStyle(.plain)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 16)
        .padding(.vertical, 14)
        .background(Color.clear)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(lightPurple, lineWidth: 2)
        )
    }

    private func remainingTimeText(until expiry: Date, now: Date) -> String {
        let seconds = max(0, Int(expiry.timeIntervalSince(now)))
        return String(format: "%02d:%02d", seconds / 60, seconds % 60)
    }

    private func isNotificationOwner(_ active: ActiveBeGitTime) -> Bool {
        guard let userID = authState.githubUser?.id else { return false }
        return active.sentBy == Int64(userID) || (active.sentBy == 0 && active.notificationID == 0)
    }

    private var endedChallengeCard: some View {
        let accentPurple = Color(red: 0.72, green: 0.58, blue: 0.98)
        let lightPurple = Color(red: 0.90, green: 0.84, blue: 1.00)

        return VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .firstTextBaseline, spacing: 10) {
                Image(systemName: "checkmark.circle")
                    .font(.system(size: 18, weight: .semibold))
                    .foregroundStyle(accentPurple)
                    .frame(width: 24)

                Text("BeGit Time")
                    .font(.system(size: 20, weight: .bold, design: .monospaced))
                    .foregroundStyle(.white)

                Text("終了")
                    .font(.system(size: 13, weight: .bold))
                    .foregroundStyle(accentPurple)

                Spacer(minLength: 8)
            }

            Text("このTimeは停止されました。投稿された内容はTimelineに残っています。")
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(.white.opacity(0.68))
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 16)
        .padding(.vertical, 14)
        .background(Color.clear)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(lightPurple, lineWidth: 2)
        )
    }

    private var achievedMemberLogins: Set<String> {
        if viewModel.notificationMemberStatuses.isEmpty == false {
            let completedIDs = Set(viewModel.notificationMemberStatuses.compactMap { status -> Int64? in
                let normalized = status.status.lowercased().replacingOccurrences(of: " ", with: "_")
                guard normalized == "on_time" || normalized == "late" else { return nil }
                return status.id
            })
            return Set(viewModel.repository.members.compactMap { member in
                guard let backendUserID = member.backendUserID,
                      completedIDs.contains(backendUserID) else { return nil }
                return member.login
            })
        }

        let activeWindow = viewModel.activeChallenge.map {
            ActiveBeGitTime(
                notificationID: $0.notificationID,
                sentBy: $0.issuer.userID,
                sentAt: $0.sentAt,
                expiresAt: $0.endsAt
            )
        }
        guard let challenge = activeWindow ?? viewModel.activeBeGitTime ?? viewModel.endedBeGitTime else {
            return []
        }

        return Set(viewModel.activities.filter {
            $0.backendPostID != nil &&
            $0.date >= challenge.sentAt &&
            $0.date < challenge.expiresAt
        }.map(\.author.login))
    }

    private var shouldDimUnachievedMembers: Bool {
        viewModel.activeChallenge != nil ||
        viewModel.activeBeGitTime != nil ||
        viewModel.endedBeGitTime != nil ||
        viewModel.notificationMemberStatuses.isEmpty == false
    }

    //  下部固定エリア背景
    private var bottomBarBackground: some View {
        LinearGradient(
            colors: [AppTheme.background.opacity(0.70), AppTheme.background],
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea(edges: .bottom)
    }

    private func statusText(_ text: String) -> some View {
        Text(text)
            .font(.system(size: 13, weight: .semibold, design: .monospaced))
            .foregroundStyle(.white.opacity(0.62))
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.vertical, 8)
    }
}

struct RepositoryDashboardView_Previews: PreviewProvider {
    static var previews: some View {
        NavigationStack {
            RepositoryDashboardView(repository: Repository.mockRepositories[0])
        }
        .environmentObject(AuthState.shared)
        .previewDevice("iPhone 16 Pro Max")
    }
}
