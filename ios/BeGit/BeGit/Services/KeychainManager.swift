//  KeychainManager.swift
//  GitHubアクセストークンをKeychainで管理するクラス

import Foundation
import Security

//  Keychain操作のインターフェース
protocol KeychainManaging: Sendable {
    func saveAccessToken(_ token: String) throws    //  保存
    func readAccessToken() throws -> String?        //  取得
    func deleteAccessToken() throws                 //  削除
}
//  Keychainを使ってアクセストークンを管理
struct KeychainManager: KeychainManaging {
    //  保存データを識別するキー
    private let service = "com.Palm7710.BeGit.auth"
    private let account = "github_access_token"

    nonisolated init() {}

    //  アクセストークンを保存
    func saveAccessToken(_ token: String) throws {
        let data = Data(token.utf8)

        //  既存トークンを削除
        try deleteAccessTokenIfPresent()

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: data
        ]

        //  新しいトークンを保存
        let status = SecItemAdd(query as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw KeychainError.saveFailed(status)
        }
    }

    //  保存済みトークンを読み込み
    func readAccessToken() throws -> String? {
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
            let data = item as? Data,
            let token = String(data: data, encoding: .utf8)
        else {
            throw KeychainError.invalidData
        }

        return token
    }

    //  保存済みトークンを削除
    func deleteAccessToken() throws {
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

    //  保存前に既存トークンがあれば削除
    private func deleteAccessTokenIfPresent() throws {
        do {
            try deleteAccessToken()
        } catch let error as KeychainError {
            if case .deleteFailed = error {
                throw error
            }
        }
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
