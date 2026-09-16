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
    //  ② Nice Work! の下書き投稿ID。指定時は新規投稿を作らず、この下書きに写真を付けて確定する
    let draftPostID: Int64?

    init(
        mainImage: UIImage?,
        frontImage: UIImage?,
        repositoryID: Int64,
        repoFullName: String,
        githubLogin: String,
        accessToken: String,
        draftPostID: Int64? = nil
    ) {
        self.mainImage = mainImage
        self.frontImage = frontImage

        self.repositoryID = repositoryID
        self.repoFullName = repoFullName
        self.githubLogin = githubLogin
        self.accessToken = accessToken
        self.draftPostID = draftPostID
    }
    // CreatePostViewModel.swift に追加

    @Published var isPosting = false
    @Published var postError: Error?
    @Published private(set) var postedActivity: RepositoryActivity?
    private var draftPhotosUploaded = false

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

        //  ② Nice Work!：下書きに写真を付けてから確定する。写真が付かなければ確定しない（下書きが残り再試行できる）
        if let draftPostID {
            //  確定だけ失敗して Post を押し直した場合に、写真が二重に付かないようにする
            if draftPhotosUploaded == false {
                try await uploadPhotosWithRetry(
                    api: api,
                    postID: draftPostID,
                    mainData: mainData,
                    frontData: frontData
                )
                draftPhotosUploaded = true
            }
            let trimmedBody = bodyText.trimmingCharacters(in: .whitespacesAndNewlines)
            try await api.confirmPost(
                repositoryID: repositoryID,
                postID: draftPostID,
                body: trimmedBody.isEmpty ? nil : trimmedBody,
                accessToken: accessToken
            )
            return
        }

        let postID = try await api.createPost(
            repositoryID: repositoryID,
            body: bodyText,
            repoFullName: repoFullName,
            githubLogin: githubLogin,
            accessToken: accessToken
        )

        // If both attempts fail, the post will remain without photos.
        // TODO: Implement deletePost API and call it here to clean up orphaned posts.
        try await uploadPhotosWithRetry(api: api, postID: postID, mainData: mainData, frontData: frontData)
    }

    //  写真アップロードを失敗時に1回だけ再試行する
    private func uploadPhotosWithRetry(
        api: BeGitBackendAPI,
        postID: Int64,
        mainData: Data,
        frontData: Data?
    ) async throws {
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
