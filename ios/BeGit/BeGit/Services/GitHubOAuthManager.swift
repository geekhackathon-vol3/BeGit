//  GitHubOAuthManager.swift
//  GitHub OAuth認証の開始からログイン完了までを管理

import AuthenticationServices
import Combine
import Foundation
import UIKit

//  GitHubログイン機能のインターフェース
protocol GitHubOAuthManaging: AnyObject, ObservableObject {
    var activeAlert: OAuthAlertContext? { get }
    func startLogin()
}

//  GitHub OAuth認証を管理するクラス
@MainActor
final class GitHubOAuthManager: NSObject, GitHubOAuthManaging {
    static let shared = GitHubOAuthManager()

    @Published private(set) var activeAlert: OAuthAlertContext? //  画面へ表示するエラー情報

    private let clientID = "Ov23li1y8CYBKRtrFgzW"       //  GitHub OAuth Client ID
    private let callbackScheme = "begit"                //  OAuthコールバック用URLスキーム
    private let callbackHost = "oauth-callback"         //  OAuthコールバック用ホスト名
    private let redirectURI = "begit://oauth-callback"  //  GitHubに登録したリダイレクトURI
    private let authBaseURL = URL(string: "https://github.com/login/oauth/authorize")!

    private var authState: AuthState?
    private var authAPI: any AuthAPI = MockAuthAPI()
    private var keychainManager: any KeychainManaging = KeychainManager()
    private var authenticationSession: ASWebAuthenticationSession?
    private var currentOAuthState = UUID().uuidString   //  ログインごとに新しいstateを生成

    //  OAuthに必要な依存オブジェクトを設定
    func configure(
        authState: AuthState,
        authAPI: any AuthAPI,
        keychainManager: any KeychainManaging
    ) {
        self.authState = authState
        self.authAPI = authAPI
        self.keychainManager = keychainManager
    }

    //  GitHubログイン開始
    func startLogin() {
        guard authenticationSession == nil else { return }

        currentOAuthState = UUID().uuidString
        guard let authorizationURL = makeAuthorizationURL() else {
            activeAlert = .init(error: GitHubOAuthError.invalidAuthorizationURL)
            return
        }

        //  GitHub認証セッションを生成
        let session = ASWebAuthenticationSession(
            url: authorizationURL,
            callbackURLScheme: callbackScheme
        ) { [weak self] callbackURL, error in
            Task { @MainActor in
                guard let self else { return }
                self.authenticationSession = nil

                if let error = error {
                    self.activeAlert = .init(error: self.mapAuthenticationError(error))
                    return
                }

                do {
                    let code = try self.extractCode(from: callbackURL)
                    let response = try await self.authAPI.exchangeCode(code: code)  //  認証コードをアクセストークンへ交換
                    try self.keychainManager.saveAccessToken(response.accessToken)  //  アクセストークンをKeychainへ保存
                    try self.keychainManager.saveGitHubUser(response.githubUser)     //  ユーザー情報をKeychainへ保存
                    self.authState?.completeLogin(response: response)               //  ログイン状態へ更新
                } catch {
                    self.activeAlert = .init(error: self.mapFlowError(error))
                }
            }
        }

        session.presentationContextProvider = self
        session.prefersEphemeralWebBrowserSession = true    //  Cookieを共有しないプライベート認証セッション
        authenticationSession = session

        if !session.start() {
            authenticationSession = nil
            activeAlert = .init(error: GitHubOAuthError.sessionStartFailed)
        }
    }

    //  エラーメッセージをクリア
    func clearAlert() {
        activeAlert = nil
    }

    //  GitHub認証URL生成
    private func makeAuthorizationURL() -> URL? {
        var components = URLComponents(url: authBaseURL, resolvingAgainstBaseURL: false)
        let scopes = ["read:user", "user:email", "repo"].joined(separator: " ")

        components?.queryItems = [
            URLQueryItem(name: "client_id", value: clientID),
            URLQueryItem(name: "redirect_uri", value: redirectURI),
            URLQueryItem(name: "scope", value: scopes),
            URLQueryItem(name: "state", value: currentOAuthState),
            URLQueryItem(name: "allow_signup", value: "true")
        ]

        return components?.url
    }

    //  コールバックURLから認証コードを取得
    private func extractCode(from callbackURL: URL?) throws -> String {
        guard let callbackURL else {
            throw GitHubOAuthError.invalidCallbackURL
        }

        guard
            callbackURL.scheme == callbackScheme,
            callbackURL.host == callbackHost
        else {
            throw GitHubOAuthError.invalidCallbackURL
        }

        let components = URLComponents(url: callbackURL, resolvingAgainstBaseURL: false)
        let state = components?.queryItems?.first(where: { $0.name == "state" })?.value
        guard state == currentOAuthState else {
            throw GitHubOAuthError.stateMismatch(expected: currentOAuthState, actual: state)
        }

        guard let code = components?.queryItems?.first(where: { $0.name == "code" })?.value,
              !code.isEmpty else {
            throw GitHubOAuthError.missingCode
        }

        return code
    }

    private func mapAuthenticationError(_ error: Error) -> Error {
        let nsError = error as NSError
        if nsError.domain == ASWebAuthenticationSessionError.errorDomain,
           nsError.code == ASWebAuthenticationSessionError.canceledLogin.rawValue {
            return GitHubOAuthError.loginCanceled
        }

        return error
    }

    private func mapFlowError(_ error: Error) -> Error {
        if error is GitHubOAuthError || error is KeychainError {
            return error
        }

        return GitHubOAuthError.networkFailure(underlying: error)
    }
}

extension GitHubOAuthManager: ASWebAuthenticationPresentationContextProviding {
    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        if let keyWindow = (
            UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap(\.windows)
            .first(where: \.isKeyWindow)
        ) {
            return keyWindow
        }

        if let windowScene = (
            UIApplication.shared.connectedScenes
            .compactMap({ $0 as? UIWindowScene })
            .first
        ) {
            return ASPresentationAnchor(windowScene: windowScene)
        }

        return ASPresentationAnchor()
    }
}

//  エラー表示用データ
struct OAuthAlertContext: Identifiable {
    let id = UUID()
    let title: String
    let message: String

    init(error: Error) {
        if let localized = error as? LocalizedError {
            title = "ログインに失敗しました"
            message = localized.errorDescription ?? "不明なエラーが発生しました。"
        } else {
            title = "ログインに失敗しました"
            message = error.localizedDescription
        }
    }
}

//   GitHub OAuth専用エラー
enum GitHubOAuthError: LocalizedError {
    case invalidAuthorizationURL
    case invalidCallbackURL
    case missingCode
    case stateMismatch(expected: String, actual: String?)
    case loginCanceled
    case sessionStartFailed
    case networkFailure(underlying: Error)

    var errorDescription: String? {
        switch self {
        case .invalidAuthorizationURL:
            return "GitHub認証URLの生成に失敗しました。"
        case .invalidCallbackURL:
            return "コールバックURLが不正です。"
        case .missingCode:
            return "認証コードを取得できませんでした。"
        case let .stateMismatch(expected, actual):
            return "OAuth state の検証に失敗しました。expected=\(expected), actual=\(actual ?? "nil")"
        case .loginCanceled:
            return "GitHubログインがキャンセルされました。"
        case .sessionStartFailed:
            return "認証セッションを開始できませんでした。"
        case .networkFailure:
            return "認証情報の取得に失敗しました。通信状態を確認してください。"
        }
    }
}
