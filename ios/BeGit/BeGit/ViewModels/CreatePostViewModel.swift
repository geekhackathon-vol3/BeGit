//  CreatePostViewModel.swift
//  投稿作成画面の入力状態管理

import Foundation
import SwiftUI
import UIKit
import Combine

@MainActor
final class CreatePostViewModel: ObservableObject {

    @Published var mainImage: UIImage?
    @Published var frontImage: UIImage?

    @Published var bodyText = ""

    let repositoryID: Int64
    let repoFullName: String
    let githubLogin: String
    let accessToken: String

    init(
        mainImage: UIImage?,
        frontImage: UIImage?,
        repositoryID: Int64,
        repoFullName: String,
        githubLogin: String,
        accessToken: String
    ) {
        self.mainImage = mainImage
        self.frontImage = frontImage

        self.repositoryID = repositoryID
        self.repoFullName = repoFullName
        self.githubLogin = githubLogin
        self.accessToken = accessToken
    }
    // CreatePostViewModel.swift に追加

    @Published var isPosting = false
    @Published var postError: Error?
    @Published private(set) var postedActivity: RepositoryActivity?

    func submitPost() async throws {
        guard !isPosting else { return }
        isPosting = true
        defer { isPosting = false }

        //  デモリポジトリ（backendID < 0）はAPI呼び出しをスキップして即時activity生成
        if repositoryID < 0 {
            postedActivity = makeDemoActivity()
            return
        }

        let api = BeGitBackendAPI()

        guard let mainImage,
              let mainData = mainImage.jpegData(compressionQuality: 0.8)
        else { throw BeGitAPIError.invalidResponse }

        let frontData = frontImage?.jpegData(compressionQuality: 0.8)

        let postID = try await api.createPost(
            repositoryID: repositoryID,
            body: bodyText,
            repoFullName: repoFullName,
            githubLogin: githubLogin,
            accessToken: accessToken
        )

        // Retry photo upload once if it fails. If both attempts fail, the post will remain without photos.
        // TODO: Implement deletePost API and call it here to clean up orphaned posts.
        do {
            try await api.uploadPhotos(
                repositoryID: repositoryID,
                postID: postID,
                mainImageData: mainData,
                frontImageData: frontData,
                accessToken: accessToken
            )
        } catch {
            // Retry once
            try await api.uploadPhotos(
                repositoryID: repositoryID,
                postID: postID,
                mainImageData: mainData,
                frontImageData: frontData,
                accessToken: accessToken
            )
        }
    }

    //  デモ用：撮影画像を temp ファイルに保存して即時表示できる RepositoryActivity を生成
    private func makeDemoActivity() -> RepositoryActivity {
        let tmp = FileManager.default.temporaryDirectory
        var mainURL: URL? = nil
        var frontURL: URL? = nil
        if let data = mainImage?.jpegData(compressionQuality: 0.85) {
            let url = tmp.appendingPathComponent(UUID().uuidString + ".jpg")
            try? data.write(to: url)
            mainURL = url
        }
        if let data = frontImage?.jpegData(compressionQuality: 0.85) {
            let url = tmp.appendingPathComponent(UUID().uuidString + ".jpg")
            try? data.write(to: url)
            frontURL = url
        }
        let avatarURL = URL(string: "https://github.com/\(githubLogin).png")
        return RepositoryActivity(
            type: .commit,
            title: repoFullName,
            comment: bodyText.isEmpty ? nil : bodyText,
            mainPhotoURL: mainURL,
            frontPhotoURL: frontURL,
            author: RepositoryMember(login: githubLogin, avatarURL: avatarURL)
        )
    }
}
