//  RepositoryListView.swift
//  認証後に表示するRepository Home画面

import SwiftUI
import UIKit

@MainActor
struct RepositoryListView: View {
    @EnvironmentObject private var authState: AuthState         //  アプリ全体で共有される認証状態
    @StateObject private var viewModel: RepositoryListViewModel //  Repository一覧状態を管理するViewModel
    @State private var navigationPath = NavigationPath()        //  Repository Home以降のpush遷移状態

    //  デフォルトViewModelで初期化
    init() {
        _viewModel = StateObject(wrappedValue: RepositoryListViewModel())
    }

    //  外部ViewModel注入用
    init(viewModel: RepositoryListViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
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
                            //  Header表示
                            headerSection
                                .padding(.bottom, 10)

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
                ToolbarItem(placement: .topBarTrailing) {
                    //  ログアウト
                    Button("Log Out", action: authState.logout)
                        .font(.system(size: 13, weight: .bold, design: .monospaced))
                        .foregroundStyle(AppTheme.accent)
                }
            }
            //  Repository追加Sheet
            .sheet(isPresented: $viewModel.isShowingAddRepository) {
                AddRepositoryView { repository in
                    //  Repository一覧へ追加
                    viewModel.addRepository(repository)
                }
            }
            .navigationDestination(for: RepositoryNavigationRoute.self) { route in
                destination(for: route)
            }
        }
        .tint(AppTheme.accent)
    }

    // MARK: - Components

    //  Header表示
    private var headerSection: some View {
        VStack(alignment: .center, spacing: 18) {
            //  BeGitロゴ表示
            logoView

            //  Home画面タイトル
            Text("Repository Home")
                .font(.system(size: 13, weight: .bold, design: .monospaced))
                .foregroundStyle(AppTheme.accent)
                .textCase(.uppercase)

            //  ログインユーザー向け説明文
            Text("Welcome, \(authState.githubUser?.login ?? "Developer"). Track your active repositories from one place.")
                .font(.system(size: 14, weight: .medium, design: .monospaced))
                .foregroundStyle(.white.opacity(0.62))
                .lineSpacing(4)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
        }
        .frame(maxWidth: .infinity)
    }

    //  BeGitロゴView
    private var logoView: some View {
        Group {
            //  ロゴ画像が存在する場合
            if let image = UIImage(named: "begit_logo") {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFit()
            } else {
                //  ロゴ画像未設定時のFallback表示
                Text("BG")
                    .font(.system(size: 18, weight: .black, design: .monospaced))
                    .foregroundStyle(.black)
            }
        }
        .frame(width: 160, height: 56)  //  ロゴサイズ
    }

    //  Repository追加ボタン
    private var addRepositoryButton: some View {
        PrimaryButton("リポジトリの追加", systemImage: "plus", action: viewModel.showAddRepository)
            .accessibilityIdentifier("add_repository_button")
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
        }
    }
}

//  BeGit共通テーマカラー
enum AppTheme {
    //  アプリ背景色
    static let background = Color(red: 0.149, green: 0.157, blue: 0.188)
    //  カード背景色
    static let cardBackground = Color(red: 0.07, green: 0.06, blue: 0.11)
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
