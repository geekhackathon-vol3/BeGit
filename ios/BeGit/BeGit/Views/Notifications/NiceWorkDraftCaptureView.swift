//  NiceWorkDraftCaptureView.swift
//  Nice Work! で確定した報告に写真を後から添付する画面。

import SwiftUI

struct NiceWorkDraftCaptureView: View {
    let groupId: Int
    let draftPostId: Int
    let onPostCompleted: () -> Void

    @EnvironmentObject private var authState: AuthState

    private enum LoadState {
        case loading
        case loaded(DraftPost)
        case failed(String)
    }

    @State private var loadState: LoadState = .loading

    var body: some View {
        Group {
            switch loadState {
            case .loading:
                ProgressView()
                    .tint(.white)
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .background(AppTheme.background)
            case .loaded(let draft):
                if let githubLogin = authState.githubUser?.login,
                   githubLogin.isEmpty == false,
                   let accessToken = authState.accessToken,
                   accessToken.isEmpty == false {
                    CameraView(
                        repositoryID: Int64(groupId),
                        repoFullName: draft.repoFullName,
                        githubLogin: githubLogin,
                        accessToken: accessToken,
                        initialPostType: draft.postType,
                        draftPostID: draft.id
                    ) { _ in
                        onPostCompleted()
                    }
                } else {
                    message("ログイン状態を確認できません。再ログインしてください。")
                }
            case .failed(let text):
                message(text, retry: true)
            }
        }
        .task { await loadDraft() }
    }

    private func loadDraft() async {
        guard let accessToken = authState.accessToken, accessToken.isEmpty == false else {
            loadState = .failed("ログイン状態を確認できません。再ログインしてください。")
            return
        }

        loadState = .loading
        do {
            let draft = try await BeGitBackendAPI().getDraftPost(
                repositoryID: Int64(groupId),
                postID: Int64(draftPostId),
                accessToken: accessToken
            )
            loadState = .loaded(draft)
        } catch {
            loadState = .failed("報告の取得に失敗しました。")
        }
    }

    private func message(_ text: String, retry: Bool = false) -> some View {
        VStack(spacing: 20) {
            Text(text)
                .font(.system(size: 16, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white.opacity(0.7))
                .multilineTextAlignment(.center)

            if retry {
                PrimaryButton("再試行", systemImage: "arrow.clockwise") {
                    Task { await loadDraft() }
                }
            }
        }
        .padding(24)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(AppTheme.background)
    }
}
