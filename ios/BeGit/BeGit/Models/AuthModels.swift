//  AuthModals.swift
//  GitHubログイン後に取得する認証情報とユーザー情報を表すデータモデル

import Foundation

//  OAuth認証が成功したときのレスポンス
struct AuthResponse: Sendable {
    let accessToken: String     //  GitHub APIを呼ぶための認証トークン
    let githubUser: GitHubUser  //  ログインしたユーザー情報
}

//  GitHubユーザーを表すモデル
struct GitHubUser: Identifiable, Equatable, Sendable {
    let id: Int         //  GitHub上のユーザーID
    let login: String   //  GitHubユーザー名
    let avatarURL: URL? //  プロフィール画像URL
    let email: String?  //  GitHubメールアドレス
}
