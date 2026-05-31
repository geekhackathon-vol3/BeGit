//  RepositoryDashboardView.swift
//  Repository選択後のDashboard / Timeline画面

import SwiftUI

@MainActor
struct RepositoryDashboardView: View {
    //  Dashboard画面の状態を管理するViewModel
    @StateObject private var viewModel: RepositoryDashboardViewModel

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

                        //  activity card一覧
                        ForEach(viewModel.activities) { activity in
                            RepositoryActivityCardView(activity: activity)
                        }
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
        }
        .toolbar(.hidden, for: .tabBar)
        .tint(AppTheme.accent)
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

    //  BeGit投稿達成済みmember ID一覧
    private var achievedMemberIDs: Set<UUID> {
        Set(
            viewModel.activities
                .filter { $0.reaction == .check }
                .map(\.author.id)
        )
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

struct RepositoryDashboardView_Previews: PreviewProvider {
    static var previews: some View {
        NavigationStack {
            RepositoryDashboardView(repository: Repository.mockRepositories[0])
        }
        .previewDevice("iPhone 16 Pro Max")
    }
}
