//  LoginViewModel.swift
//  LoginView専用のViewModel

import Combine
import Foundation

//  ログイン画面の状態と処理を管理
@MainActor
final class LoginViewModel: ObservableObject {
    @Published var alertContext: OAuthAlertContext?     //  LoginViewへ表示するエラー情報

    private let oauthManager: GitHubOAuthManager        //  GitHub OAuth認証を管理するサービス
    private var cancellables = Set<AnyCancellable>()    //  Combineの購読管理 

    //  OAuthManagerと連携設定
    init(oauthManager: GitHubOAuthManager) {
        self.oauthManager = oauthManager

        //  OAuthManagerのエラーをViewModelへ同期
        oauthManager.$activeAlert
            .receive(on: RunLoop.main)
            .assign(to: &$alertContext)
    }

    //  デフォルト構成のViewModel生成
    static func makeDefault() -> LoginViewModel {
        LoginViewModel(oauthManager: .shared)
    }

    //  GitHubログイン開始
    func signInWithGitHub() {
        oauthManager.startLogin()
    }

    //  エラーダイアログを閉じる
    func dismissAlert() {
        oauthManager.clearAlert()
    }
}
