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

    func submitPost() async throws {
        isPosting = true
        defer { isPosting = false }

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

        try await api.uploadPhotos(
            repositoryID: repositoryID,
            postID: postID,
            mainImageData: mainData,
            frontImageData: frontData,
            accessToken: accessToken
        )
    }
}
