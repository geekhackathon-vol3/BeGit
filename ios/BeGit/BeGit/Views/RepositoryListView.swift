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
                                SwipeToDeleteRepositoryRow(
                                    repository: repository,
                                    onOpen: {
                                        navigationPath.append(RepositoryNavigationRoute.dashboard(repository))
                                    }
                                ) {
                                    viewModel.removeRepository(repository)
                                }
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
                    viewModel: AddRepositoryViewModel(
                        accessToken: authState.accessToken,
                        existingRepositories: viewModel.repositories
                    )
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
        VStack(spacing: 12) {
            PrimaryButton("リポジトリの追加", systemImage: "plus", action: viewModel.showAddRepository)
                .accessibilityIdentifier("add_repository_button")
        }
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
                Text(displayedGitHubUserName)
                    .font(.system(size: 13, weight: .black, design: .monospaced))
                    .foregroundStyle(.white)

                Text(displayedUserIDText)
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
            name: "Guest",
            avatarURL: nil,
            email: nil
        )
    }

    private var displayedGitHubUserName: String {
        if let name = displayedGitHubUser.name, name.isEmpty == false {
            return name
        }

        return displayedGitHubUser.login
    }

    private var displayedUserIDText: String {
        guard let githubUser = authState.githubUser else {
            return "-"
        }

        return displayedGitHubUserID
    }

    private var displayedGitHubUserID: String {
        let login = displayedGitHubUser.login
        guard let first = login.first else {
            return "-"
        }

        return first.uppercased() + login.dropFirst()
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
        case .camera(let notification):
                CameraView(
                    repositoryID: notification.repository.backendID ?? 0,
                    repoFullName: notification.repository.name,
                    githubLogin: authState.githubUser?.login ?? "",
                    accessToken: authState.accessToken ?? ""
                ) {
                    // 投稿完了後 → Result画面へ
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

//  左スワイプで削除buttonを表示するRepository行
private struct SwipeToDeleteRepositoryRow: View {
    let repository: Repository
    let onOpen: () -> Void
    let onDelete: () -> Void

    @State private var offsetX: CGFloat = 0

    private let deleteRevealWidth: CGFloat = 86
    private let deleteButtonSize: CGFloat = 58
    private let rowCornerRadius: CGFloat = 10

    var body: some View {
        ZStack(alignment: .trailing) {
            deleteButton
                .padding(.trailing, 10)
                .scaleEffect(isDeleteVisible ? 1 : 0.62)
                .opacity(isDeleteVisible ? 1 : 0)
                .animation(.spring(response: 0.30, dampingFraction: 0.46), value: isDeleteVisible)

            RepositoryCardView(repository: repository)
                .contentShape(RoundedRectangle(cornerRadius: rowCornerRadius, style: .continuous))
                .offset(x: offsetX)
                .highPriorityGesture(swipeGesture)
                .onTapGesture {
                    if isDeleteVisible {
                        withAnimation(.spring(response: 0.32, dampingFraction: 0.56)) {
                            offsetX = 0
                        }
                    } else {
                        onOpen()
                    }
                }
                .animation(.spring(response: 0.34, dampingFraction: 0.62), value: offsetX)
        }
    }

    private var deleteButton: some View {
        Button {
            withAnimation(.spring(response: 0.24, dampingFraction: 0.54)) {
                offsetX = -deleteRevealWidth - 10
            }

            DispatchQueue.main.asyncAfter(deadline: .now() + 0.12) {
                withAnimation(.spring(response: 0.26, dampingFraction: 0.70)) {
                    onDelete()
                }
            }
        } label: {
            Image(systemName: "trash.fill")
                .font(.system(size: 22, weight: .black))
                .foregroundStyle(.white)
                .frame(width: deleteButtonSize, height: deleteButtonSize)
                .background(Color(.systemRed))
                .clipShape(Circle())
                .shadow(color: Color(.systemRed).opacity(0.35), radius: 10, x: 0, y: 4)
        }
        .buttonStyle(.plain)
        .accessibilityLabel("\(repository.name)を削除")
    }

    private var swipeGesture: some Gesture {
        DragGesture(minimumDistance: 12, coordinateSpace: .local)
            .onChanged { value in
                let horizontalMovement = value.translation.width
                guard abs(horizontalMovement) > abs(value.translation.height) else { return }

                if horizontalMovement < 0 {
                    offsetX = max(horizontalMovement, -deleteRevealWidth)
                } else if isDeleteVisible {
                    offsetX = min(-deleteRevealWidth + horizontalMovement, 0)
                }
            }
            .onEnded { value in
                let shouldReveal = value.translation.width < -38 || value.predictedEndTranslation.width < -deleteRevealWidth

                withAnimation(.spring(response: 0.36, dampingFraction: 0.48)) {
                    offsetX = shouldReveal ? -deleteRevealWidth : 0
                }
            }
    }

    private var isDeleteVisible: Bool {
        offsetX < -12
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
