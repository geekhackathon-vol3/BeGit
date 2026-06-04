//  RepositoryListView.swift
//  認証後に表示するRepository Home画面

import SwiftUI
import UIKit

@MainActor
struct RepositoryListView: View {
    @EnvironmentObject private var authState: AuthState         //  アプリ全体で共有される認証状態
    @StateObject private var viewModel: RepositoryListViewModel //  Repository一覧状態を管理するViewModel
    @ObservedObject private var notificationRouter = NotificationRouter.shared //  通知タップ由来の遷移要求
    @State private var navigationPath = NavigationPath()        //  Repository Home以降のpush遷移状態
    private let currentUserAPI: any CurrentUserAPI

    //  デフォルトViewModelで初期化
    init(currentUserAPI: any CurrentUserAPI = BeGitBackendAPI()) {
        _viewModel = StateObject(wrappedValue: RepositoryListViewModel())
        self.currentUserAPI = currentUserAPI
    }

    //  外部ViewModel注入用
    init(viewModel: RepositoryListViewModel, currentUserAPI: any CurrentUserAPI = BeGitBackendAPI()) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.currentUserAPI = currentUserAPI
    }

    var body: some View {
        //  Navigation対応Home画面
        NavigationStack(path: $navigationPath) {
            ZStack {
                //  背景色
                AppTheme.background
                    .ignoresSafeArea()

                VStack(spacing: 0) {
                    ScrollView {
                        //  Repository card一覧
                        LazyVStack(alignment: .leading, spacing: 16) {
                            Text("Repositories")
                                .font(.custom("Bitcount", size: 34))
                                .foregroundStyle(.white)
                                .frame(maxWidth: .infinity, alignment: .leading)

                            //  ログイン中ユーザー情報
                            loggedInUserSummary
                                .padding(.bottom, 2)

                            if viewModel.isLoading {
                                statusText("Loading repositories...")
                            }

                            if let errorMessage = viewModel.errorMessage {
                                statusText(errorMessage)
                            }

                            //  Repository一覧表示
                            ForEach(viewModel.repositories) { repository in
                                NavigationLink(value: RepositoryNavigationRoute.dashboard(repository)) {
                                    RepositoryCardView(repository: repository)
                                }
                                .buttonStyle(.plain)
                            }
                        }
                        .padding(.horizontal, 20)
                        .padding(.top, 24)
                        .padding(.bottom, 108)  //  下部button領域分の余白
                    }

                    //  Repository追加button
                    addRepositoryButton
                        .padding(.horizontal, 20)
                        .padding(.top, 14)
                        .padding(.bottom, 18)
                        .background(bottomBarBackground)    //  bottom bar背景
                }
            }
            .navigationBarTitleDisplayMode(.inline)
            //  NavigationBar items
            .toolbar {
                ToolbarItem(placement: .principal) {
                    BeGitToolbarLogoView()
                }

                ToolbarItem(placement: .topBarTrailing) {
                    //  ログアウト
                    Button("Log Out", action: authState.logout)
                        .font(.system(size: 13, weight: .bold, design: .monospaced))
                        .foregroundStyle(AppTheme.accent)
                }
            }
            //  Repository追加Sheet
            .sheet(isPresented: $viewModel.isShowingAddRepository) {
                AddRepositoryView(
                    viewModel: AddRepositoryViewModel(accessToken: authState.accessToken)
                ) { repository in
                    //  Repository一覧へ追加
                    viewModel.addRepository(repository)
                }
                .environmentObject(authState)
            }
            .navigationDestination(for: RepositoryNavigationRoute.self) { route in
                destination(for: route)
            }
            .task {
                await refreshLoggedInUserIfNeeded(accessToken: authState.accessToken)
                await viewModel.loadRepositories(accessToken: authState.accessToken)
            }
            .onChange(of: authState.accessToken) { _, accessToken in
                Task {
                    await refreshLoggedInUserIfNeeded(accessToken: accessToken)
                    await viewModel.loadRepositories(accessToken: accessToken)
                }
            }
            //  通知タップ由来の遷移要求を navigationPath へ反映する
            .onChange(of: notificationRouter.pendingRoute) { _, _ in
                applyPendingNotificationRoute()
            }
            //  アプリ未起動からの通知タップで起動した場合、表示時に拾う
            .onAppear {
                applyPendingNotificationRoute()
            }
        }
        .tint(AppTheme.accent)
    }

    //  共有 Router に積まれた通知 route を push し、消費済みにする
    private func applyPendingNotificationRoute() {
        guard let route = notificationRouter.pendingRoute else { return }
        navigationPath.append(route)
        notificationRouter.consume()
    }

    // MARK: - Components

    //  Repository追加ボタン
    private var addRepositoryButton: some View {
        PrimaryButton("リポジトリの追加", systemImage: "plus", action: viewModel.showAddRepository)
            .accessibilityIdentifier("add_repository_button")
    }

    //  ログイン中ユーザー情報表示
    private var loggedInUserSummary: some View {
        HStack(alignment: .center, spacing: 10) {
            AvatarView(
                member: RepositoryMember(
                    login: displayedGitHubUser.login,
                    avatarURL: displayedGitHubUser.avatarURL
                ),
                size: 34
            )
            .background(
                Circle()
                    .fill(AppTheme.background.opacity(0.82))
            )
            .overlay(
                Circle()
                    .stroke(Color.white.opacity(0.72), lineWidth: 1.5)
            )

            VStack(alignment: .leading, spacing: 3) {
                Text(displayedGitHubUser.login)
                    .font(.system(size: 13, weight: .black, design: .monospaced))
                    .foregroundStyle(.white)

                Text(displayedGitHubUserIDText)
                    .font(.system(size: 11, weight: .semibold, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.64))
            }

            Spacer()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    //  表示用ユーザー
    private var displayedGitHubUser: GitHubUser {
        authState.githubUser ?? GitHubUser(
            id: 0,
            login: "Guest",
            avatarURL: nil,
            email: nil
        )
    }

    private var displayedGitHubUserIDText: String {
        guard let githubUser = authState.githubUser else {
            return "ID: -"
        }

        return "ID: \(githubUser.id)"
    }

    //  下部固定エリア背景
    private var bottomBarBackground: some View {
        LinearGradient(
            colors: [
                //  上部透明背景
                AppTheme.background.opacity(0.70),
                //  下部背景色
                AppTheme.background
            ],
            startPoint: .top,
            endPoint: .bottom
        )
        //  SafeArea下部まで背景を拡張
        .ignoresSafeArea(edges: .bottom)
    }

    private func statusText(_ text: String) -> some View {
        Text(text)
            .font(.system(size: 13, weight: .semibold, design: .monospaced))
            .foregroundStyle(.white.opacity(0.62))
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.vertical, 8)
    }

    private func refreshLoggedInUserIfNeeded(accessToken: String?) async {
        guard authState.githubUser == nil,
              let accessToken,
              accessToken.isEmpty == false else {
            return
        }

        do {
            let githubUser = try await currentUserAPI.getCurrentUser(accessToken: accessToken)
            authState.updateGitHubUser(githubUser)
        } catch {
            // Repository一覧取得側で認証エラーを表示するため、ここではユーザー補完だけを諦める
        }
    }

    //  routeに応じて遷移先Viewを生成
    @ViewBuilder
    private func destination(for route: RepositoryNavigationRoute) -> some View {
        switch route {
        //  Repository Dashboard画面へ遷移
        case .dashboard(let repository):
            RepositoryDashboardView(repository: repository)
        //  通知作成画面へ遷移
        case .makeNotification(let repository):
            MakeNotificationView(repository: repository) { notification in
                navigationPath.append(RepositoryNavigationRoute.notificationResult(notification))
            }
        //  通知結果画面へ遷移
        case .notificationResult(let notification):
            NotificationResultView(notification: notification) {
                //  NavigationStackをrootまで戻す
                navigationPath.removeLast(navigationPath.count)
            }

        // MARK: - FCM 通知タップからの遷移（#55・中身は #56/#57 が実装）
        case let .notificationPostCreation(groupId, notificationId):
            NotificationPostCreationStubView(groupId: groupId, notificationId: notificationId)
        case let .notificationNiceWorkDraft(groupId, draftPostId, status):
            NotificationNiceWorkDraftStubView(groupId: groupId, draftPostId: draftPostId, status: status)
        case let .notificationChallengeResult(groupId, notificationId):
            NotificationChallengeResultStubView(groupId: groupId, notificationId: notificationId)
        case let .notificationSprintOverview(groupId, sprintId):
            NotificationSprintOverviewStubView(groupId: groupId, sprintId: sprintId)
        case let .notificationSprintResult(groupId, sprintId):
            NotificationSprintResultStubView(groupId: groupId, sprintId: sprintId)
        case let .notificationPostDetail(groupId, postId, kind):
            NotificationPostDetailStubView(groupId: groupId, postId: postId, kind: kind)
        }
    }
}

//  BeGit共通テーマカラー
enum AppTheme {
    //  アプリ背景色
    static let background = Color(red: 0.149, green: 0.157, blue: 0.188)
    //  カード背景色
    static let cardBackground = Color(red: 0.07, green: 0.06, blue: 0.11)
    //  Repository一覧カード背景色
    static let repositoryCardBackground = Color(red: 0.267, green: 0.267, blue: 0.267)
    //  入力欄背景色
    static let fieldBackground = Color.white.opacity(0.07)
    //  メインアクセントカラー
    static let accent = Color(red: 0.804, green: 0.718, blue: 0.965)
    //  Dashboard / Notificationで使うsoft pink
    static let softPink = Color(red: 1.00, green: 0.72, blue: 0.84)
}

//  iPhone SE Preview
struct RepositoryListView_iPhoneSE_Previews: PreviewProvider {
    static var previews: some View {
        RepositoryListView()
            .environmentObject(AuthState.shared)
            .previewDevice("iPhone SE (3rd generation)")
    }
}

//  iPhone 16 Pro Max Preview
struct RepositoryListView_iPhone16ProMax_Previews: PreviewProvider {
    static var previews: some View {
        RepositoryListView()
            .environmentObject(AuthState.shared)
            .previewDevice("iPhone 16 Pro Max")
    }
}
