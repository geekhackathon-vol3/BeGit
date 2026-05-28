//  RepositoryNavigationRoute.swift
//  Repository Home以降のpush遷移を表すroute

import Foundation

//  Repository画面遷移route
enum RepositoryNavigationRoute: Hashable, Sendable {
    //  Repository Dashboard画面
    case dashboard(Repository)
    //  通知作成画面
    case makeNotification(Repository)
    //  通知結果画面
    case notificationResult(RepositoryNotification)
}

