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
            //  背景色
            AppTheme.background
                .ignoresSafeArea()

            if isShowingGallery {
                RepositoryPhotoGalleryContentView(
                    repository: viewModel.repository,
                    activities: viewModel.activities
                )
                .transition(.opacity)
            } else {
                VStack(spacing: 0) {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 18) {
                            //  Timeline Header
                            timelineHeader

                            //  進行中のBeGit Time（残り時間・発行者・撮影導線・終了ボタン）
                            if let challenge = viewModel.activeChallenge {
                                ActiveChallengeBannerView(
                                    challenge: challenge,
                                    repository: viewModel.repository,
                                    isEnding: viewModel.isEndingChallenge,
                                    onEnd: {
                                        Task { await viewModel.endChallenge(accessToken: authState.accessToken) }
                                    },
                                    onExpire: {
                                        Task { await viewModel.loadActiveChallenge(accessToken: authState.accessToken) }
                                    }
                                )
                                .transition(.opacity)
                            }

                            if let challengeErrorMessage = viewModel.challengeErrorMessage {
                                statusText(challengeErrorMessage)
                            }

                            if viewModel.isLoading {
                                statusText("Loading timeline...")
                            }

                            if let errorMessage = viewModel.errorMessage {
                                statusText(errorMessage)
                            }

                            //  達成状況プログレスバー
                            progressSummary

                            //  activity card一覧（横幅フル）
                            RepositoryActivityTimelineView(
                                activities: viewModel.activities,
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
                        .padding(.bottom, 104)
                    }

                    //  通知作成画面へ遷移（BeGit Time 進行中は発行できないので無効化）
                    Group {
                        if viewModel.activeChallenge != nil {
                            PrimaryCapsuleButtonLabel(
                                title: "BeGit Time 進行中",
                                systemImage: "hourglass",
                                isEnabled: false
                            )
                            .accessibilityLabel("BeGit Time進行中のため通知を作成できません")
                        } else {
                            NavigationLink(value: RepositoryNavigationRoute.makeNotification(viewModel.repository)) {
                                PrimaryCapsuleButtonLabel(
                                    title: "通知を作成する",
                                    systemImage: "bolt.badge.clock",
                                    isEnabled: true
                                )
                            }
                            .buttonStyle(.plain)
                        }
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 14)
                    .padding(.bottom, 18)
                    .background(bottomBarBackground)
                }
                .transition(.opacity)
            }
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
        .toolbar(.hidden, for: .tabBar)
        .tint(AppTheme.accent)
        //  accessToken変更時に前のタスクを自動キャンセルしてリロード
        .task(id: authState.accessToken) {
            await viewModel.loadActivities(
                accessToken: authState.accessToken,
                currentUserID: authState.githubUser.map { Int64($0.id) }
            )
        }
        //  表示のたび（通知作成・撮影から戻ったときを含む）に進行中のBeGit Timeを取り直す
        .onAppear {
            Task { await viewModel.loadActiveChallenge(accessToken: authState.accessToken) }
        }
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

    //  Repository member表示エリア
    private var memberStrip: some View {
        HStack(spacing: 10) {
            //  member avatar一覧
            MemberAvatarRowView(
                members: viewModel.repository.members,
                avatarSpacing: 6,
                achievedMemberIDs: achievedMemberIDs
            )

            //  member数表示
            Text("\(viewModel.repository.memberCount) members")
                .font(.system(size: 12, weight: .bold, design: .monospaced))
                .foregroundStyle(AppTheme.softPink)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    //  達成状況サマリー（Result画面と同一スタイル）
    private var progressSummary: some View {
        VStack(alignment: .leading, spacing: 14) {
            MemberAvatarRowView(members: viewModel.repository.members, avatarSize: 42)

            GeometryReader { proxy in
                ZStack(alignment: .leading) {
                    Capsule()
                        .fill(Color.white.opacity(0.12))
                    Capsule()
                        .fill(AppTheme.accent)
                        .frame(width: proxy.size.width * viewModel.progress)
                    Text(viewModel.progressText)
                        .appFont(.body)
                        .foregroundStyle(Color.black.opacity(0.76))
                        .lineLimit(1)
                        .minimumScaleFactor(0.78)
                        .frame(maxWidth: .infinity, alignment: .center)
                }
            }
            .frame(height: 30)
        }
    }

    //  Timelineにactivityがあるmember ID一覧
    private var achievedMemberIDs: Set<UUID> {
        Set(viewModel.activities.map(\.author.id))
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
