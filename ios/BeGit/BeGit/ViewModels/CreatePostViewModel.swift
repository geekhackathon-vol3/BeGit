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
    @Published var selectedType: RepositoryActivityType
    @Published var contentSource: PostContentSource
    @Published var selectedCommit: GitHubCommitSelection?
    @Published var selectedPullRequest: GitHubPullRequestSelection?
    @Published private(set) var recentCommits: [GitHubCommitSelection] = []
    @Published private(set) var recentPullRequests: [GitHubPullRequestSelection] = []
    @Published private(set) var isLoadingGitHubActivities = false
    @Published var githubActivityError: Error?

    let repositoryID: Int64
    let repoFullName: String
    let githubLogin: String
    let accessToken: String
    let notificationID: Int64?
    //  既存の報告ID。指定時は新規投稿を作らず写真を添付する
    let draftPostID: Int64?
    let isPostTypeSelectionEnabled: Bool

    init(
        mainImage: UIImage?,
        frontImage: UIImage?,
        repositoryID: Int64,
        repoFullName: String,
        githubLogin: String,
        accessToken: String,
        notificationID: Int64? = nil,
        initialPostType: RepositoryActivityType = .commit,
        draftPostID: Int64? = nil
    ) {
        self.mainImage = mainImage
        self.frontImage = frontImage

        self.repositoryID = repositoryID
        self.repoFullName = repoFullName
        self.githubLogin = githubLogin
        self.accessToken = accessToken
        self.notificationID = notificationID
        self.selectedType = notificationID != nil && draftPostID == nil ? .memo : initialPostType
        self.contentSource = notificationID != nil && draftPostID == nil ? .photo : (initialPostType == .memo ? .manual : .github)
        self.draftPostID = draftPostID
        self.isPostTypeSelectionEnabled = draftPostID == nil
    }
    @Published var isPosting = false
    @Published var postError: Error?
    @Published private(set) var postedActivity: RepositoryActivity?
    private var draftPhotosUploaded = false

    var canSubmit: Bool {
        guard isPosting == false else { return false }
        if draftPostID != nil {
            return true
        }
        if contentSource == .photo { return mainImage != nil }
        if selectedType == .memo || contentSource == .manual {
            return bodyText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty == false && isPosting == false
        }
        return switch selectedType {
        case .commit: selectedCommit != nil
        case .pullRequest: selectedPullRequest != nil
        case .memo: false
        }
    }

    var selectedGitHubActivityTitle: String? {
        return switch selectedType {
        case .commit: selectedCommit?.message
        case .pullRequest:
            selectedPullRequest.map { "PR #\($0.number): \($0.title)" }
        case .memo: nil
        }
    }

    func selectPostType(_ type: RepositoryActivityType) {
        guard isPostTypeSelectionEnabled else { return }
        selectedType = type
        selectedCommit = nil
        selectedPullRequest = nil
        githubActivityError = nil
        contentSource = type == .memo ? .manual : .github
    }

    func selectContentSource(_ source: PostContentSource) {
        guard selectedType != .memo else { return }
        contentSource = source
        if source == .manual {
            clearSelectedGitHubActivity()
        }
        githubActivityError = nil
    }

    func clearSelectedGitHubActivity() {
        selectedCommit = nil
        selectedPullRequest = nil
    }

    func loadGitHubActivities() async {
        guard contentSource == .github, selectedType != .memo else { return }
        isLoadingGitHubActivities = true
        githubActivityError = nil
        defer { isLoadingGitHubActivities = false }

        if repositoryID < 0 {
            if selectedType == .commit {
                recentCommits = [
                    GitHubCommitSelection(
                        sha: "demo123", message: "feat: デモ用の最新commit", authorLogin: githubLogin,
                        date: ISO8601DateFormatter().string(from: Date()), additions: 24, deletions: 3
                    )
                ]
            } else {
                recentPullRequests = [
                    GitHubPullRequestSelection(
                        number: 42, title: "デモ用のPull Request", authorLogin: githubLogin,
                        state: "open", merged: false, updatedAt: ISO8601DateFormatter().string(from: Date())
                    )
                ]
            }
            return
        }

        do {
            let api = BeGitBackendAPI()
            switch selectedType {
            case .commit:
                recentCommits = try await api.listRecentCommits(
                    repositoryID: repositoryID,
                    githubLogin: githubLogin,
                    accessToken: accessToken
                )
            case .pullRequest:
                recentPullRequests = try await api.listRecentPullRequests(
                    repositoryID: repositoryID,
                    githubLogin: githubLogin,
                    accessToken: accessToken
                )
            case .memo:
                break
            }
        } catch {
            githubActivityError = error
        }
    }

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
              let mainData = Self.timelineImageData(mainImage, maxDimension: 1_280)
        else { throw BeGitAPIError.invalidResponse }

        let frontData = frontImage.flatMap { Self.timelineImageData($0, maxDimension: 640) }

        //  既存の報告に写真を添付する。
        if let draftPostID {
            //  Post を押し直した場合に写真を二重に送らない。
            if draftPhotosUploaded == false {
                try await uploadPhotosWithRetry(
                    api: api,
                    postID: draftPostID,
                    mainData: mainData,
                    frontData: frontData
                )
                draftPhotosUploaded = true
            }
            // Result画面へ遷移した直後から、この投稿に対して
            // リアクションAPIを呼べるようバックエンドの投稿IDを保持する。
            postedActivity = makeDemoActivity(backendPostID: draftPostID)
            return
        }

        let postID = try await api.createPost(
            repositoryID: repositoryID,
            notificationID: notificationID,
            body: bodyText,
            repoFullName: repoFullName,
            githubLogin: githubLogin,
            postType: selectedType,
            contentSource: contentSource,
            commitSHA: selectedType == .commit ? selectedCommit?.sha : nil,
            pullRequestNumber: selectedType == .pullRequest ? selectedPullRequest?.number : nil,
            accessToken: accessToken
        )

        // 写真報告はこのアップロード成功時にサーバーが確定する。
        try await uploadPhotosWithRetry(api: api, postID: postID, mainData: mainData, frontData: frontData)

        // Result画面へ戻った直後にも、選択した投稿タイプを表示する。
        // 次のフィード取得が完了すると、サーバーの正規データへ置き換わる。
        // createPostが返したIDを渡し、即時表示中もリアクションAPIの対象にする。
        postedActivity = makeDemoActivity(backendPostID: postID)
    }

    // Timeline表示サイズに合わせてアップロード前に縮小する。
    // 巨大なカメラ原本をR2・通信・画像decodeへ持ち込まない。
    private static func timelineImageData(_ image: UIImage, maxDimension: CGFloat) -> Data? {
        let longest = max(image.size.width, image.size.height)
        let rendered: UIImage
        if longest > maxDimension {
            let scale = maxDimension / longest
            let target = CGSize(width: image.size.width * scale, height: image.size.height * scale)
            rendered = image.preparingThumbnail(of: target) ?? image
        } else {
            rendered = image
        }
        return rendered.jpegData(compressionQuality: 0.78)
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
    private func makeDemoActivity(backendPostID: Int64? = nil) -> RepositoryActivity {
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
            backendPostID: backendPostID,
            type: selectedType,
            title: selectedGitHubActivityTitle ?? (bodyText.isEmpty ? repoFullName : bodyText),
            comment: bodyText.isEmpty ? nil : bodyText,
            mainPhotoURL: mainURL,
            frontPhotoURL: frontURL,
            author: RepositoryMember(login: githubLogin, avatarURL: avatarURL)
        )
    }
}
