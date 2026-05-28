//
//  ContentView.swift
//  BeGit
//
//  Created by palm on 2026/05/24.
//

import SwiftUI

struct ContentView: View {
    // EnvironmentObject（どの画面からでも受け取れる）として AuthState を取得
    @EnvironmentObject private var authState: AuthState

    var body: some View {
        Group {
            if authState.isLoggedIn {
                MainTabView()   // ログイン済み
            } else {
                LoginView()     // 未ログイン
            }
        }
    }
}

struct ContentView_Previews: PreviewProvider {
    static var previews: some View {
        ContentView()
            .environmentObject(AuthState.shared)
    }
}
