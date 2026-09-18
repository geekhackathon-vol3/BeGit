//  GitHubActivityPickerView.swift
//  投稿に紐づける最近の commit / PR を選ぶシート

import SwiftUI

struct GitHubActivityPickerView: View {
    @ObservedObject var viewModel: CreatePostViewModel
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                Group {
                    if viewModel.isLoadingGitHubActivities {
                        ProgressView("GitHubから読み込み中…")
                            .tint(AppTheme.Text.primary)
                            .foregroundStyle(AppTheme.Text.primary)
                    } else if let error = viewModel.githubActivityError {
                        ContentUnavailableView {
                            Label("読み込めませんでした", systemImage: "exclamationmark.triangle")
                        } description: {
                            Text(error.localizedDescription)
                        } actions: {
                            Button("もう一度試す") {
                                Task { await viewModel.loadGitHubActivities() }
                            }
                        }
                    } else if isEmpty {
                        ContentUnavailableView(
                            emptyTitle,
                            systemImage: "point.3.connected.trianglepath.dotted",
                            description: Text("このリポジトリに自分の最近のデータがありません。")
                        )
                    } else {
                        ScrollView {
                            LazyVStack(spacing: 10) {
                                if viewModel.selectedType == .commit {
                                    ForEach(viewModel.recentCommits) { commit in
                                        commitRow(commit)
                                    }
                                } else {
                                    ForEach(viewModel.recentPullRequests) { pullRequest in
                                        pullRequestRow(pullRequest)
                                    }
                                }
                            }
                            .padding(16)
                        }
                    }
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)

                manualPostButton
                    .padding(.horizontal, 16)
                    .padding(.top, 10)
                    .padding(.bottom, 18)
            }
            .background(AppTheme.background.ignoresSafeArea())
            .navigationTitle(viewModel.selectedType == .commit ? "commitを選ぶ" : "PRを選ぶ")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        dismiss()
                    } label: {
                        Image(systemName: "xmark")
                            .font(.system(size: 15, weight: .semibold))
                            .frame(width: 32, height: 32)
                    }
                    .accessibilityLabel("閉じる")
                    .foregroundStyle(AppTheme.Text.primary)
                }
            }
        }
        .preferredColorScheme(.dark)
        .task(id: viewModel.selectedType) {
            await viewModel.loadGitHubActivities()
        }
    }

    private var isEmpty: Bool {
        if viewModel.selectedType == .commit {
            return viewModel.recentCommits.isEmpty
        }
        return viewModel.recentPullRequests.isEmpty
    }

    private var emptyTitle: String {
        viewModel.selectedType == .commit ? "commitがありません" : "PRがありません"
    }

    private var manualPostButton: some View {
        Button {
            viewModel.selectContentSource(.manual)
            dismiss()
        } label: {
            Label("GitHubデータを使わずコメントのみ", systemImage: "text.bubble")
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(.white.opacity(0.88))
                .frame(maxWidth: .infinity)
                .frame(height: 48)
                .background(Color.white.opacity(0.09))
                .clipShape(Capsule())
        }
        .buttonStyle(.plain)
    }

    private func commitRow(_ commit: GitHubCommitSelection) -> some View {
        Button {
            viewModel.selectedCommit = commit
            dismiss()
        } label: {
            activityRow(
                icon: "checkmark.seal.fill",
                title: commit.message,
                detail: "\(String(commit.sha.prefix(7)))  +\(commit.additions)  −\(commit.deletions)",
                isSelected: viewModel.selectedCommit?.sha == commit.sha,
                tint: RepositoryActivityType.commit.pickerTint
            )
        }
        .buttonStyle(.plain)
    }

    private func pullRequestRow(_ pullRequest: GitHubPullRequestSelection) -> some View {
        Button {
            viewModel.selectedPullRequest = pullRequest
            dismiss()
        } label: {
            activityRow(
                icon: "arrow.triangle.pull",
                title: "PR #\(pullRequest.number): \(pullRequest.title)",
                detail: pullRequest.merged ? "Merged" : pullRequest.state.capitalized,
                isSelected: viewModel.selectedPullRequest?.number == pullRequest.number,
                tint: RepositoryActivityType.pullRequest.pickerTint
            )
        }
        .buttonStyle(.plain)
    }

    private func activityRow(
        icon: String,
        title: String,
        detail: String,
        isSelected: Bool,
        tint: Color
    ) -> some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
                .font(.system(size: 17, weight: .bold))
                .foregroundStyle(tint)
                .frame(width: 28)

            VStack(alignment: .leading, spacing: 5) {
                Text(title)
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(.white)
                    .multilineTextAlignment(.leading)
                    .lineLimit(2)
                Text(detail)
                    .font(.system(size: 12, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.55))
            }

            Spacer(minLength: 8)
            if isSelected {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(tint)
            }
        }
        .padding(14)
        .background(Color.white.opacity(0.09))
        .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
    }
}
