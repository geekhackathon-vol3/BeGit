//  RepositoryDashboardView.swift
//  Repository選択後のDashboard / Timeline画面

import SwiftUI

@MainActor
struct RepositoryDashboardView: View {
    @EnvironmentObject private var authState: AuthState
    //  Dashboard画面の状態を管理するViewModel
    @StateObject private var viewModel: RepositoryDashboardViewModel
    @State private var showRepoSetting = false

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

            VStack(spacing: 0) {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 18) {
                        //  Timeline Header
                        timelineHeader

                        //  Repository member一覧
                        memberStrip

                        if viewModel.isLoading {
                            statusText("Loading timeline...")
                        }

                        if let errorMessage = viewModel.errorMessage {
                            statusText(errorMessage)
                        }

                        //  達成状況プログレスバー
                        progressSummary

                        //  activity card一覧（横幅フル）
                        RepositoryActivityTimelineView(activities: viewModel.activities)
                            .padding(.horizontal, -20)
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 20)
                    .padding(.bottom, 104)
                }

                //  通知作成画面へ遷移
                NavigationLink(value: RepositoryNavigationRoute.makeNotification(viewModel.repository)) {
                    PrimaryCapsuleButtonLabel(
                        title: "通知を作成する",
                        systemImage: "bolt.badge.clock",
                        isEnabled: true
                    )
                }
                .buttonStyle(.plain)
                .padding(.horizontal, 20)
                .padding(.top, 14)
                .padding(.bottom, 18)
                .background(bottomBarBackground)
            }
        }
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
                Button {
                    showRepoSetting = true
                } label: {
                    Image(systemName: "gearshape.fill")
                        .foregroundStyle(AppTheme.softPink)
                        .frame(minWidth: 44, minHeight: 44)
                }
                .accessibilityLabel("リポジトリ設定")
            }
        }
        .toolbar(.hidden, for: .tabBar)
        .tint(AppTheme.accent)
        //  accessToken変更時に前のタスクを自動キャンセルしてリロード
        .task(id: authState.accessToken) {
            await viewModel.loadActivities(accessToken: authState.accessToken)
        }
        .sheet(isPresented: $showRepoSetting) {
            RepoSettingView(repository: viewModel.repository)
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

    //  activityに登録されているユニークメンバー（モック含む、アバターURL確実）
    private var uniqueActivityMembers: [RepositoryMember] {
        var seen = Set<String>()
        return viewModel.activities.compactMap { activity in
            guard !seen.contains(activity.author.login) else { return nil }
            seen.insert(activity.author.login)
            return activity.author
        }
    }

    //  達成状況サマリー（Result画面と同一スタイル）
    private var progressSummary: some View {
        VStack(alignment: .leading, spacing: 14) {
            MemberAvatarRowView(members: uniqueActivityMembers, avatarSize: 42)

            GeometryReader { proxy in
                ZStack(alignment: .leading) {
                    Capsule()
                        .fill(Color.white.opacity(0.12))
                    Capsule()
                        .fill(AppTheme.accent)
                        .frame(width: proxy.size.width * viewModel.progress)
                    Text(viewModel.progressText)
                        .font(.system(size: 14, weight: .black, design: .monospaced))
                        .foregroundStyle(.black)
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
