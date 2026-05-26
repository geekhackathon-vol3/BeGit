//  MainTabView.swift
//  ログイン後に表示するメイン画面

import SwiftUI

//  ログイン後のメイン画面
struct MainTabView: View {
    @EnvironmentObject private var authState: AuthState //  アプリ全体で共有される認証状態

    var body: some View {
        //  タブ形式の画面
        TabView {
            //  画面遷移用のナビゲーションコンテナ
            NavigationStack {
                VStack(spacing: 20) {
                    //  GitHubユーザー名を表示
                    Text("Welcome, \(authState.githubUser?.login ?? "Developer")")
                        .font(.title2.weight(.bold))

                    //  ログイン成功メッセージ
                    Text("GitHub OAuth login completed.")
                        .foregroundStyle(.secondary)
                    
                    //  ログアウト
                    Button("Log Out") {
                        authState.logout()
                    }
                    .buttonStyle(.borderedProminent)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .padding()
                .navigationTitle("BeGit")   //  ナビゲーションバータイトル
            }
            .tabItem {
                //  Homeタブ
                Label("Home", systemImage: "house")
            }
        }
    }
}

struct MainTabView_Previews: PreviewProvider {
    static var previews: some View {
        MainTabView()
            .environmentObject(AuthState.shared)
    }
}
