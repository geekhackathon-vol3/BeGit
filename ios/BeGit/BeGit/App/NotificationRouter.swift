//  NotificationRouter.swift
//  UIKit 側（AppDelegate の通知ハンドラ）と SwiftUI 側（NavigationStack）を繋ぐ橋。
//  AppDelegate が通知タップを parse して pendingRoute に書き込み、RepositoryListView が
//  これを監視して navigationPath に反映する。アプリ全体で1つだけ共有する。

import Combine
import Foundation

@MainActor
final class NotificationRouter: ObservableObject {
    static let shared = NotificationRouter()

    //  未処理の遷移先。View が消費したら nil に戻す。
    @Published var pendingRoute: RepositoryNavigationRoute?

    private init() {}

    //  通知タップ時に遷移要求を積む。アプリ未起動からの起動でも、View 側が
    //  onChange / onAppear で拾えるよう @Published に保持しておく。
    func requestRoute(_ route: RepositoryNavigationRoute) {
        pendingRoute = route
    }

    //  View が遷移を消費したら呼ぶ。
    func consume() {
        pendingRoute = nil
    }
}
