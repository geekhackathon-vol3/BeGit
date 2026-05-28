//  MainTabView.swift
//  ログイン後に表示するメイン画面

import SwiftUI

//  ログイン後のメイン画面
struct MainTabView: View {
    var body: some View {
        //  タブ形式の画面
        TabView {
            //  Repository一覧画面
            RepositoryListView()
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
