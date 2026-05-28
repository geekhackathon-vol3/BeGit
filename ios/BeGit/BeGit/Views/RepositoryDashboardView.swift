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

struct RepositoryDashboardView_Previews: PreviewProvider {
    static var previews: some View {
        NavigationStack {
            RepositoryDashboardView(repository: Repository.mockRepositories[0])
        }
        .previewDevice("iPhone 16 Pro Max")
    }
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
                        //  共通Header
                        BeGitHeaderView(title: "Timeline", subtitle: viewModel.repository.name)

                        //  Repository member一覧
                        memberStrip

                        //  Timeline Section title
                        SectionTitleView("Timeline", caption: "commit / PR / sorry activity")

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
        .toolbar(.hidden, for: .tabBar)
        .tint(AppTheme.accent)
    }

    // MARK: - Components

    //  Repository member表示エリア
    private var memberStrip: some View {
        HStack {
            //  member avatar一覧
            MemberAvatarRowView(members: viewModel.repository.members)

            Spacer()

            //  member数表示
            Text("\(viewModel.repository.memberCount) members")
                .font(.system(size: 12, weight: .bold, design: .monospaced))
                .foregroundStyle(AppTheme.softPink)
                .padding(.horizontal, 12)
                .padding(.vertical, 8)
                .background(AppTheme.softPink.opacity(0.10))
                .clipShape(Capsule())
        }
        .padding(14)
        .background(Color.white.opacity(0.05))
        .clipShape(RoundedRectangle(cornerRadius: 22, style: .continuous))
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
