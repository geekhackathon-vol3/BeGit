//  RepoSettingView.swift
//  リポジトリ設定シート（リポジトリ情報・メンバー一覧・GitHub認証）

import SwiftUI

@MainActor
struct RepoSettingView: View {
    let repository: Repository
    @ObservedObject private var oauthManager = GitHubOAuthManager.shared
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            ZStack {
                AppTheme.background.ignoresSafeArea()

                ScrollView {
                    VStack(alignment: .leading, spacing: 24) {
                        // MARK: GitHub認証ヘッダーボタン
                        githubAuthButton

                        // MARK: リポジトリ情報
                        settingSection(title: "REPOSITORY") {
                            repoInfoRow
                        }

                        // MARK: メンバー一覧
                        settingSection(title: "MEMBERS") {
                            VStack(spacing: 0) {
                                ForEach(Array(repository.members.enumerated()), id: \.element.id) { index, member in
                                    memberRow(member)
                                    if index < repository.members.count - 1 {
                                        Divider()
                                            .background(Color.white.opacity(0.08))
                                            .padding(.leading, 56)
                                    }
                                }
                            }
                            .background(Color.white.opacity(0.05))
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                        }

                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 24)
                    .padding(.bottom, 40)
                }
            }
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button {
                        oauthManager.startLogin()
                    } label: {
                        Image(systemName: "person.badge.key.fill")
                            .foregroundStyle(AppTheme.softPink)
                            .frame(minWidth: 44, minHeight: 44)
                    }
                    .accessibilityLabel("GitHubで認証")
                }
                ToolbarItem(placement: .principal) {
                    Text("設定")
                        .font(.system(size: 14, weight: .bold, design: .monospaced))
                        .foregroundStyle(.white)
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        dismiss()
                    } label: {
                        Image(systemName: "xmark")
                            .foregroundStyle(AppTheme.softPink)
                            .frame(minWidth: 44, minHeight: 44)
                    }
                }
            }
            .toolbarBackground(AppTheme.background, for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
            .alert(item: Binding(
                get: { oauthManager.activeAlert },
                set: { _ in oauthManager.clearAlert() }
            )) { alertContext in
                Alert(
                    title: Text(alertContext.title),
                    message: Text(alertContext.message),
                    dismissButton: .default(Text("OK"))
                )
            }
        }
    }

    // MARK: - Components

    private var githubAuthButton: some View {
        Button {
            oauthManager.startLogin()
        } label: {
            HStack(spacing: 10) {
                Image(systemName: "person.badge.key.fill")
                    .font(.system(size: 16, weight: .bold))
                Image(systemName: "arrow.right")
                    .font(.system(size: 13, weight: .bold))
                Text("GitHubで再認証")
                    .font(.system(size: 13, weight: .bold, design: .monospaced))
                Spacer()
            }
            .foregroundStyle(AppTheme.softPink)
            .padding(.horizontal, 16)
            .padding(.vertical, 14)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Color.white.opacity(0.06))
            .overlay(alignment: .leading) {
                Rectangle()
                    .frame(width: 3)
                    .foregroundStyle(AppTheme.softPink)
            }
            .clipShape(RoundedRectangle(cornerRadius: 12))
        }
        .buttonStyle(.plain)
        .accessibilityLabel("GitHubで認証")
    }

    private var repoInfoRow: some View {
        HStack(spacing: 12) {
            // オーナーアバター
            Group {
                if let url = repository.ownerAvatarURL {
                    AsyncImage(url: url) { phase in
                        switch phase {
                        case .success(let image):
                            image.resizable().scaledToFill()
                        default:
                            placeholderAvatar
                        }
                    }
                } else {
                    placeholderAvatar
                }
            }
            .frame(width: 40, height: 40)
            .clipShape(Circle())

            // リポジトリ名
            Text(repository.name)
                .font(.system(size: 14, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white)
                .lineLimit(1)
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Color.white.opacity(0.05))
        .clipShape(RoundedRectangle(cornerRadius: 12))
    }

    private var placeholderAvatar: some View {
        Image("github_default_icon")
            .resizable()
            .scaledToFill()
    }

    private func memberRow(_ member: RepositoryMember) -> some View {
        HStack(spacing: 12) {
            AvatarView(member: member, size: 36)
            Text(member.login)
                .font(.system(size: 13, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white.opacity(0.85))
            Spacer()
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 12)
    }

    @ViewBuilder
    private func settingSection(title: String, @ViewBuilder content: () -> some View) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(title)
                .font(.system(size: 11, weight: .bold, design: .monospaced))
                .foregroundStyle(AppTheme.softPink)
            content()
        }
    }
}

// MARK: - Preview

struct RepoSettingView_Previews: PreviewProvider {
    static var previews: some View {
        RepoSettingView(repository: Repository.mockRepositories[0])
            .previewDevice("iPhone 16 Pro Max")
    }
}
