//  NotificationResultView.swift
//  通知送信後のMock結果画面

import SwiftUI

@MainActor
struct NotificationResultView: View {
    //  通知結果画面の状態を管理するViewModel
    @StateObject private var viewModel: NotificationResultViewModel
    @EnvironmentObject private var authState: AuthState     //  アクセストークン取得用
    @State private var isShowingGallery = false
    @State private var isShowingRepoSetting = false
    @State private var activityToDelete: RepositoryActivity?
    @State private var isShowingDeleteConfirmation = false
    @State private var deleteErrorMessage = ""
    @State private var isShowingDeleteError = false
    @State private var isShowingStopConfirmation = false
    @State private var stopErrorMessage = ""
    @State private var isShowingStopError = false
    let onReturnHome: () -> Void    //  通知結果画面の状態を管理するViewModel

    //  通知モデルからViewModelを生成
    init(notification: RepositoryNotification, justPostedActivity: RepositoryActivity? = nil, onReturnHome: @escaping () -> Void) {
        _viewModel = StateObject(wrappedValue: NotificationResultViewModel(notification: notification, justPostedActivity: justPostedActivity))
        self.onReturnHome = onReturnHome
    }

    //  外部ViewModel注入用
    init(viewModel: NotificationResultViewModel, onReturnHome: @escaping () -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onReturnHome = onReturnHome
    }

    var body: some View {
        ZStack {
            //  背景色
            AppTheme.background
                .ignoresSafeArea()

            if isShowingGallery {
                RepositoryPhotoGalleryContentView(
                    repository: viewModel.notification.repository,
                    activities: viewModel.activities
                )
                .transition(.opacity)
            } else {
                VStack(spacing: 0) {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 18) {
                            if let active = viewModel.activeBeGitTime {
                                TimelineView(.periodic(from: .now, by: 1)) { context in
                                    if context.date < active.expiresAt {
                                        activeChallengeBanner(
                                            active,
                                            now: context.date,
                                            canStop: isNotificationOwner(active),
                                            onStop: { isShowingStopConfirmation = true }
                                        )
                                    } else {
                                        endedChallengeBanner
                                    }
                                }
                            } else if viewModel.endedBeGitTime != nil {
                                endedChallengeBanner
                            }

                            //  Result Header
                            resultHeader

            //  通知結果サマリー
            resultSummary

                            //  Activity一覧（横幅フル）
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
                        .padding(.horizontal, 20)
                        .padding(.top, 20)
                        .padding(.bottom, 104)  //  下部固定button領域分の余白
                    }

                    //  ホームへ戻るbutton
                    PrimaryButton("ホームへ戻る", systemImage: "house.fill", action: onReturnHome)
                        .padding(.horizontal, 20)
                        .padding(.top, 14)
                        .padding(.bottom, 18)
                        .background(bottomBarBackground)    //  下部固定エリア背景
                }
                .transition(.opacity)
            }
        }
        .animation(.easeInOut(duration: 0.20), value: isShowingGallery)
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
                        Image(systemName: isShowingGallery ? "doc.text.fill" : "square.grid.3x3.fill")
                            .foregroundStyle(AppTheme.softPink)
                            .frame(width: 44, height: 44)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel(isShowingGallery ? "リザルトに戻る" : "投稿写真一覧")

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
            RepoSettingView(repository: viewModel.notification.repository)
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
        .task {
            //  実写真付きフィードを取得して Timeline を差し替える
            await viewModel.loadActivities(
                accessToken: authState.accessToken,
                currentUserID: authState.githubUser.map { Int64($0.id) }
            )
            await viewModel.loadNotificationStatus(accessToken: authState.accessToken)
        }
    }

    // MARK: - Components

    //  Result画面Header
    private var resultHeader: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Result")
                .appFont(.title)
                .foregroundStyle(AppTheme.Text.primary)
                .frame(maxWidth: .infinity, alignment: .leading)

            Text(viewModel.notification.repository.name)
                .appFont(.sectionHeader)
                .foregroundStyle(AppTheme.Text.low)
                .lineLimit(1)
        }
    }

    private func activeChallengeBanner(
        _ active: ActiveBeGitTime,
        now: Date,
        canStop: Bool,
        onStop: @escaping () -> Void
    ) -> some View {
        let accentPurple = Color(red: 0.72, green: 0.58, blue: 0.98)
        let lightPurple = Color(red: 0.90, green: 0.84, blue: 1.00)
        let issuer = viewModel.members.first(where: {
            $0.backendUserID == active.sentBy
        }) ?? viewModel.members.first
        let issuerLogin = issuer?.login ?? "メンバー"

        return VStack(alignment: .leading, spacing: 8) {
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

                Text(remainingTimeText(until: active.expiresAt, now: now))
                    .font(.system(size: 17, weight: .bold, design: .monospaced))
                    .foregroundStyle(AppTheme.softPink)
                    .monospacedDigit()
            }

            HStack(spacing: 10) {
                if let issuer = issuer {
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

            if canStop {
                HStack {
                    Spacer()
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

    private var endedChallengeBanner: some View {
        HStack(spacing: 10) {
            Image(systemName: "checkmark.circle")
                .foregroundStyle(AppTheme.accent)
            Text("BeGit Time 終了")
                .font(.system(size: 16, weight: .bold, design: .monospaced))
            Text("投稿された内容は残っています。")
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(AppTheme.Text.high)
        }
        .foregroundStyle(AppTheme.Text.primary)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func remainingTimeText(until expiry: Date, now: Date) -> String {
        let seconds = max(0, Int(expiry.timeIntervalSince(now)))
        return String(format: "%02d:%02d", seconds / 60, seconds % 60)
    }

    private func isNotificationOwner(_ active: ActiveBeGitTime) -> Bool {
        guard let userID = authState.githubUser?.id else { return false }
        return active.sentBy == Int64(userID) || (active.sentBy == 0 && active.notificationID == 0)
    }

    //  通知結果サマリー
    private var resultSummary: some View {
        VStack(alignment: .leading, spacing: 14) {
            BeGitTimeProgressSummaryView(
                members: viewModel.members,
                achievedMemberLogins: achievedMemberLogins,
                dimUnachieved: shouldDimUnachievedMembers,
                comment: viewModel.notification.comment
            )
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var achievedMemberLogins: Set<String> {
        if viewModel.notificationMemberStatuses.isEmpty == false {
            let completedIDs = Set(viewModel.notificationMemberStatuses.compactMap { status -> Int64? in
                let normalized = status.status.lowercased().replacingOccurrences(of: " ", with: "_")
                guard normalized == "on_time" || normalized == "late" else { return nil }
                return status.id
            })
            return Set(viewModel.members.compactMap { member in
                guard let backendUserID = member.backendUserID,
                      completedIDs.contains(backendUserID) else { return nil }
                return member.login
            })
        }

        let start = viewModel.activeBeGitTime?.sentAt
            ?? viewModel.endedBeGitTime?.sentAt
            ?? viewModel.notification.createdAt
        let end = viewModel.activeBeGitTime?.expiresAt
            ?? viewModel.endedBeGitTime?.expiresAt
            ?? start.addingTimeInterval(60 * 60)

        return Set(viewModel.activities.filter {
            $0.backendPostID != nil &&
            $0.date >= start &&
            $0.date < end
        }.map(\.author.login))
    }

    private var shouldDimUnachievedMembers: Bool {
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
}

//  NotificationResultView Preview
struct NotificationResultView_Previews: PreviewProvider {
    static var previews: some View {
        NavigationStack {
            NotificationResultView(
                notification: RepositoryNotification(
                    repository: Repository.mockRepositories[0],
                    selectedMembers: Repository.mockRepositories[0].members,
                    comment: "Mock notification comment"
                ),
                onReturnHome: {}
            )
            .environmentObject(AuthState.shared)
        }
        .previewDevice("iPhone 16 Pro Max")
    }
}
