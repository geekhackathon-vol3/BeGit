//  NotificationResultView.swift
//  通知送信後のMock結果画面

import SwiftUI

@MainActor
struct NotificationResultView: View {
    //  通知結果画面の状態を管理するViewModel
    @StateObject private var viewModel: NotificationResultViewModel
    let onReturnHome: () -> Void    //  通知結果画面の状態を管理するViewModel

    //  通知モデルからViewModelを生成
    init(notification: RepositoryNotification, onReturnHome: @escaping () -> Void) {
        _viewModel = StateObject(wrappedValue: NotificationResultViewModel(notification: notification))
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
                        //  共通Header
                        BeGitHeaderView(title: "notification result", subtitle: viewModel.notification.repository.name)

                        //  通知結果サマリー
                        resultSummary

                        //  Timeline見出し
                        SectionTitleView("Timeline", caption: "mock completion activity")

                        //  Mock activity一覧
                        ForEach(viewModel.activities) { activity in
                            RepositoryActivityCardView(activity: activity)
                        }
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
    }

    // MARK: - Components

    //  通知結果サマリー
    private var resultSummary: some View {
        VStack(alignment: .leading, spacing: 16) {
            //  通知対象member avatar一覧
            MemberAvatarRowView(members: viewModel.notification.selectedMembers, avatarSize: 42)

            //  通知コメント表示
            if viewModel.notification.comment.isEmpty == false {
                Text(viewModel.notification.comment)
                    .font(.system(size: 14, weight: .semibold, design: .monospaced))
                    .foregroundStyle(AppTheme.softPink.opacity(0.82))
                    .lineSpacing(4)
            }

            VStack(alignment: .leading, spacing: 8) {
                //  達成状況テキスト
                Text(viewModel.progressText)
                    .font(.system(size: 15, weight: .black, design: .monospaced))
                    .foregroundStyle(.white)

                //  達成率Progress bar
                GeometryReader { proxy in
                    ZStack(alignment: .leading) {
                        //  Progress bar背景
                        Capsule()
                            .fill(Color.white.opacity(0.08))

                        //  Progress bar進捗
                        Capsule()
                            .fill(AppTheme.accent)
                            .frame(width: proxy.size.width * viewModel.progress)
                    }
                }
                .frame(height: 12)
            }
        }
        .padding(16)
        .background(Color.white.opacity(0.06))                              //  card背景
        .clipShape(RoundedRectangle(cornerRadius: 24, style: .continuous))  //  card shape
        //  card border
        .overlay(
            RoundedRectangle(cornerRadius: 24, style: .continuous)
                .stroke(AppTheme.accent.opacity(0.22), lineWidth: 1)
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
        }
        .previewDevice("iPhone 16 Pro Max")
    }
}
