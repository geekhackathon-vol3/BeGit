//  KeychainManager.swift
//  GitHubアクセストークンをKeychainで管理するクラス

import Foundation
import Security

//  Keychain操作のインターフェース
protocol KeychainManaging: Sendable {
    func saveAccessToken(_ token: String) throws    //  保存
    func readAccessToken() throws -> String?        //  取得
    func saveGitHubUser(_ user: GitHubUser) throws  //  ユーザー情報保存
    func readGitHubUser() throws -> GitHubUser?      //  ユーザー情報取得
    func deleteAccessToken() throws                 //  削除
    func deleteGitHubUser() throws                  //  ユーザー情報削除
}

//  Keychainを使ってアクセストークンを管理
struct KeychainManager: KeychainManaging {
    //  保存データを識別するキー
    private let service = "com.Palm7710.BeGit.auth"
    private let tokenAccount = "github_access_token"
    private let userAccount = "github_user"

    nonisolated init() {}

    //  アクセストークンを保存
    func saveAccessToken(_ token: String) throws {
        let data = Data(token.utf8)
        try save(data: data, account: tokenAccount)
    }

    //  保存済みトークンを読み込み
    func readAccessToken() throws -> String? {
        guard let data = try readData(account: tokenAccount) else {
            return nil
        }

        guard let token = String(data: data, encoding: .utf8) else {
            throw KeychainError.invalidData
        }

        return token
    }

    //  GitHubユーザー情報を保存
    func saveGitHubUser(_ user: GitHubUser) throws {
        let data = try JSONEncoder().encode(GitHubUserKeychainItem(user: user))
        try save(data: data, account: userAccount)
    }

    //  保存済みGitHubユーザー情報を読み込み
    func readGitHubUser() throws -> GitHubUser? {
        guard let data = try readData(account: userAccount) else {
            return nil
        }

        return try JSONDecoder().decode(GitHubUserKeychainItem.self, from: data).user
    }

    //  保存済みトークンを削除
    func deleteAccessToken() throws {
        try deleteItem(account: tokenAccount)
    }

    //  保存済みGitHubユーザー情報を削除
    func deleteGitHubUser() throws {
        try deleteItem(account: userAccount)
    }

    private func save(data: Data, account: String) throws {
        try deleteItemIfPresent(account: account)

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: data
        ]

        let status = SecItemAdd(query as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw KeychainError.saveFailed(status)
        }
    }

    private func readData(account: String) throws -> Data? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]

        var item: CFTypeRef?
        //  Keychainから検索
        let status = SecItemCopyMatching(query as CFDictionary, &item)

        if status == errSecItemNotFound {
            return nil
        }

        guard status == errSecSuccess else {
            throw KeychainError.readFailed(status)
        }

        guard
            let data = item as? Data
        else {
            throw KeychainError.invalidData
        }

        return data
    }

    private func deleteItem(account: String) throws {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]

        let status = SecItemDelete(query as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw KeychainError.deleteFailed(status)
        }
    }

    private func deleteItemIfPresent(account: String) throws {
        do {
            try deleteItem(account: account)
        } catch let error as KeychainError {
            if case .deleteFailed = error {
                throw error
            }
        }
    }
}

private struct GitHubUserKeychainItem: Codable {
    let id: Int
    let login: String
    let avatarURL: URL?
    let email: String?

    init(user: GitHubUser) {
        id = user.id
        login = user.login
        avatarURL = user.avatarURL
        email = user.email
    }

    var user: GitHubUser {
        GitHubUser(id: id, login: login, avatarURL: avatarURL, email: email)
    }
}

//  Keychain操作時のエラー
enum KeychainError: LocalizedError {
    case saveFailed(OSStatus)
    case readFailed(OSStatus)
    case deleteFailed(OSStatus)
    case invalidData

    var errorDescription: String? {
        switch self {
        case .saveFailed:
            return "アクセストークンの保存に失敗しました。"
        case .readFailed:
            return "アクセストークンの読み込みに失敗しました。"
        case .deleteFailed:
            return "アクセストークンの削除に失敗しました。"
        case .invalidData:
            return "保存されたトークンの形式が不正です。"
        }
    }
}
