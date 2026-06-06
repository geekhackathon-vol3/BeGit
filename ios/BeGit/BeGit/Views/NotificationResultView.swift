//  NotificationResultView.swift
//  通知送信後のMock結果画面

import SwiftUI

@MainActor
struct NotificationResultView: View {
    //  通知結果画面の状態を管理するViewModel
    @StateObject private var viewModel: NotificationResultViewModel
    @EnvironmentObject private var authState: AuthState     //  アクセストークン取得用
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

            VStack(spacing: 0) {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 18) {
                        //  Result Header
                        resultHeader

                        //  通知結果サマリー
                        resultSummary

                        //  Activity一覧（横幅フル）
                        RepositoryActivityTimelineView(activities: viewModel.activities)
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
        .task {
            //  実写真付きフィードを取得して Timeline を差し替える
            await viewModel.loadActivities(accessToken: authState.accessToken)
        }
    }

    // MARK: - Components

    //  Result画面Header
    private var resultHeader: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Result")
                .font(.custom("Bitcount", size: 34))
                .foregroundStyle(.white)
                .frame(maxWidth: .infinity, alignment: .leading)

            Text(viewModel.notification.repository.name)
                .font(.system(size: 12, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white.opacity(0.50))
                .lineLimit(1)
        }
    }

    //  通知結果サマリー
    private var resultSummary: some View {
        VStack(alignment: .leading, spacing: 14) {
            //  通知対象member avatar一覧
            MemberAvatarRowView(members: viewModel.notification.selectedMembers, avatarSize: 42)

            //  通知コメント表示
            if viewModel.notification.comment.isEmpty == false {
                Text(viewModel.notification.comment)
                    .font(.system(size: 14, weight: .semibold, design: .monospaced))
                    .foregroundStyle(AppTheme.softPink.opacity(0.82))
                    .lineSpacing(4)
            }

            progressSummary
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    //  達成状況とProgress bar
    private var progressSummary: some View {
        //  達成率Progress bar
        GeometryReader { proxy in
            ZStack(alignment: .leading) {
                //  Progress bar背景
                Capsule()
                    .fill(Color.white.opacity(0.12))

                //  Progress bar進捗
                Capsule()
                    .fill(AppTheme.accent)
                    .frame(width: proxy.size.width * viewModel.progress)

                //  達成状況テキスト
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
