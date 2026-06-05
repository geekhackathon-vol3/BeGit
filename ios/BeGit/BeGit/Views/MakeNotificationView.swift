//  MakeNotificationView.swift
//  BeReal風Repository通知作成画面

import SwiftUI

@MainActor
struct MakeNotificationView: View {
    @EnvironmentObject private var authState: AuthState
    @StateObject private var viewModel: MakeNotificationViewModel
    @State private var isMemberSearchPresented = false
    private let onSend: (RepositoryNotification) -> Void

    init(repository: Repository, onSend: @escaping (RepositoryNotification) -> Void = { _ in }) {
        _viewModel = StateObject(wrappedValue: MakeNotificationViewModel(repository: repository))
        self.onSend = onSend
    }

    init(
        viewModel: MakeNotificationViewModel,
        onSend: @escaping (RepositoryNotification) -> Void = { _ in }
    ) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onSend = onSend
    }

    var body: some View {
        ZStack {
            AppTheme.background
                .ignoresSafeArea()

            VStack(spacing: 0) {
                ScrollView {
                    VStack(alignment: .leading, spacing: 24) {
                        makeNotificationHeader
                        membersSection
                        commentsSection

                        if let errorMessage = viewModel.errorMessage {
                            Text(errorMessage)
                                .appFont(.label)
                                .foregroundStyle(AppTheme.softPink)
                                .lineSpacing(3)
                        }
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 20)
                    .padding(.bottom, 104)
                }

                Button(action: sendNotification) {
                    PrimaryCapsuleButtonLabel(
                        title: viewModel.isSending ? "送信中..." : "通知を送る",
                        systemImage: "paperplane.fill",
                        isEnabled: viewModel.canSend
                    )
                }
                .buttonStyle(.plain)
                .disabled(viewModel.canSend == false)
                .padding(.horizontal, 20)
                .padding(.top, 14)
                .padding(.bottom, 18)
                .background(bottomBarBackground)
            }
        }
        .navigationBarTitleDisplayMode(.inline)
        .navigationBarBackButtonHidden(true)
        .toolbar {
            ToolbarItem(placement: .topBarLeading) {
                BeGitBackButton()
            }
            ToolbarItem(placement: .principal) {
                BeGitToolbarLogoView()
            }
        }
        .toolbar(.hidden, for: .tabBar)
        .tint(AppTheme.accent)
        .task {
            await viewModel.loadMembers(accessToken: authState.accessToken)
        }
        .sheet(isPresented: $isMemberSearchPresented) {
            GitHubUserSearchSheetView(
                accessToken: authState.accessToken,
                existingMembers: viewModel.members,
                repositoryMembers: viewModel.repositoryMemberCandidates
            ) { member in
                viewModel.addMember(member)
            }
        }
    }

    // MARK: - Components

    private var makeNotificationHeader: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Make Notification")
                .appFont(.title)
                .foregroundStyle(AppTheme.Text.primary)
                .frame(maxWidth: .infinity, alignment: .leading)

            Text(viewModel.repository.name)
                .appFont(.sectionHeader)
                .foregroundStyle(AppTheme.Text.low)
                .lineLimit(1)
        }
    }

    private var membersSection: some View {
        VStack(alignment: .leading, spacing: 14) {
            sectionTitle("■ Team Members")
            memberListBox
        }
    }

    private var memberListBox: some View {
        VStack(alignment: .leading, spacing: 12) {
            memberListSubheader("Selected members")

            if viewModel.isLoadingMembers {
                memberLoadingState
            } else if viewModel.members.isEmpty {
                emptyMemberState
            } else {
                VStack(spacing: 10) {
                    ForEach(viewModel.members) { member in
                        selectedMemberRow(member)
                    }
                }
            }

            addMemberButton
        }
        .frame(maxWidth: .infinity, minHeight: 54, alignment: .leading)
        .padding(14)
        .background(AppTheme.cardBackground)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(AppTheme.borderSubtle, lineWidth: 2)
        )
    }

    private var memberLoadingState: some View {
        Text("Loading members")
            .appFont(.body)
            .foregroundStyle(AppTheme.Text.disabled)
            .frame(maxWidth: .infinity, minHeight: 34, alignment: .leading)
    }

    private var emptyMemberState: some View {
        Text("No members selected")
            .appFont(.body)
            .foregroundStyle(AppTheme.Text.disabled)
            .frame(maxWidth: .infinity, minHeight: 34, alignment: .leading)
    }

    private func memberListSubheader(_ title: String) -> some View {
        Text(title)
            .appFont(.sectionHeader)
            .foregroundStyle(AppTheme.Text.muted)
            .textCase(.uppercase)
    }

    private func selectedMemberRow(_ member: RepositoryMember) -> some View {
        HStack(spacing: 12) {
            AvatarView(member: member, size: 34)

            Text(member.login)
                .appFont(.subheadline)
                .foregroundStyle(AppTheme.Text.primary)
                .lineLimit(1)

            Spacer()

            Button {
                viewModel.removeMember(member)
            } label: {
                Image(systemName: "minus")
                    .font(.system(size: 16, weight: .black))
                    .foregroundStyle(.black)
                    .frame(width: 30, height: 30)
                    .background(AppTheme.softPink)
                    .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
            }
            .buttonStyle(.plain)
            .accessibilityLabel("\(member.login)を通知対象から外す")
        }
    }

    private var addMemberButton: some View {
        Button {
            isMemberSearchPresented = true
        } label: {
            HStack(spacing: 8) {
                Image(systemName: "plus")
                    .font(.system(size: 16, weight: .black))

                Text("Add member")
                    .appFont(.label)
            }
            .foregroundStyle(.black)
            .frame(maxWidth: .infinity)
            .frame(height: 38)
            .background(AppTheme.accent)
            .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        }
        .buttonStyle(.plain)
        .accessibilityLabel("GitHubユーザーを検索してTeam Membersに追加")
    }

    private var commentsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            sectionTitle("■ Comments")

            TextEditor(text: $viewModel.comment)
                .appFont(.subheadline)
                .foregroundStyle(AppTheme.Text.primary)
                .scrollContentBackground(.hidden)
                .frame(minHeight: 132)
                .padding(12)
                .background(AppTheme.fieldBackground)
                .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
                .overlay(alignment: .topLeading) {
                    if viewModel.comment.isEmpty {
                        Text("今から実装タイムです。準備できたらpushしてね。")
                            .appFont(.body)
                            .foregroundStyle(AppTheme.Text.muted)
                            .padding(.horizontal, 18)
                            .padding(.vertical, 20)
                            .allowsHitTesting(false)
                    }
                }
        }
    }

    private var bottomBarBackground: some View {
        LinearGradient(
            colors: [AppTheme.background.opacity(0.70), AppTheme.background],
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea(edges: .bottom)
    }

    private func sectionTitle(_ title: String) -> some View {
        Text(title)
            .font(.system(size: 20, weight: .regular, design: .monospaced))
            .foregroundStyle(AppTheme.sectionPink)
    }

    private func sendNotification() {
        Task {
            guard let notification = await viewModel.sendNotification(accessToken: authState.accessToken) else { return }
            onSend(notification)
        }
    }
}

struct MakeNotificationView_Previews: PreviewProvider {
    static var previews: some View {
        NavigationStack {
            MakeNotificationView(repository: Repository.mockRepositories[0])
        }
        .environmentObject(AuthState.shared)
        .previewDevice("iPhone SE (3rd generation)")
    }
}