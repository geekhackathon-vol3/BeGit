//  MakeNotificationView.swift
//  BeReal風Repository通知作成画面

import SwiftUI

@MainActor
struct MakeNotificationView: View {
    @EnvironmentObject private var authState: AuthState
    //  通知作成画面の状態を管理するViewModel
    @StateObject private var viewModel: MakeNotificationViewModel
    @State private var isMemberSearchPresented = false
    private let onSend: (RepositoryNotification) -> Void

     //  通知作成画面の状態を管理するViewModel
    init(repository: Repository, onSend: @escaping (RepositoryNotification) -> Void = { _ in }) {
        _viewModel = StateObject(wrappedValue: MakeNotificationViewModel(repository: repository))
        self.onSend = onSend
    }

    //  外部ViewModel注入用
    init(
        viewModel: MakeNotificationViewModel,
        onSend: @escaping (RepositoryNotification) -> Void = { _ in }
    ) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onSend = onSend
    }

    var body: some View {
        ZStack {
            //  背景色
            AppTheme.background
                .ignoresSafeArea()

            VStack(spacing: 0) {
                ScrollView {
                    VStack(alignment: .leading, spacing: 24) {
                        //  通知作成Header
                        makeNotificationHeader

                        //  通知対象member選択
                        membersSection
                        //  通知コメント入力
                        commentsSection

                        if let errorMessage = viewModel.errorMessage {
                            Text(errorMessage)
                                .font(.system(size: 13, weight: .semibold, design: .monospaced))
                                .foregroundStyle(AppTheme.softPink)
                                .lineSpacing(3)
                        }
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 20)
                    .padding(.bottom, 104)  //  下部固定button領域分の余白  
                }

                //  通知を生成して結果画面へ遷移
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
                .background(bottomBarBackground)    //  下部固定エリア背景
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

    //  通知作成画面Header
    private var makeNotificationHeader: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Make Notification")
                .font(.custom("Bitcount", size: 34))
                .foregroundStyle(.white)
                .frame(maxWidth: .infinity, alignment: .leading)

            Text(viewModel.repository.name)
                .font(.system(size: 12, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white.opacity(0.50))
                .lineLimit(1)
        }
    }

    //  Team member選択Section
    private var membersSection: some View {
        VStack(alignment: .leading, spacing: 14) {
            //  Section title
            sectionTitle("■ Team Members")

            memberListBox
        }
    }

    //  member選択リスト
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
        .background(Color(red: 0.247, green: 0.247, blue: 0.286))
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(Color(red: 0.310, green: 0.322, blue: 0.357), lineWidth: 2)
        )
    }

    //  member同期中表示
    private var memberLoadingState: some View {
        Text("Loading members")
            .font(.system(size: 14, weight: .semibold, design: .monospaced))
            .foregroundStyle(.white.opacity(0.42))
            .frame(maxWidth: .infinity, minHeight: 34, alignment: .leading)
    }

    //  member未選択表示
    private var emptyMemberState: some View {
        Text("No members selected")
            .font(.system(size: 14, weight: .semibold, design: .monospaced))
            .foregroundStyle(.white.opacity(0.42))
            .frame(maxWidth: .infinity, minHeight: 34, alignment: .leading)
    }

    //  memberリスト内小見出し
    private func memberListSubheader(_ title: String) -> some View {
        Text(title)
            .font(.system(size: 12, weight: .bold, design: .monospaced))
            .foregroundStyle(.white.opacity(0.54))
            .textCase(.uppercase)
    }

    //  選択済みmember行
    private func selectedMemberRow(_ member: RepositoryMember) -> some View {
        HStack(spacing: 12) {
            AvatarView(member: member, size: 34)

            Text(member.login)
                .font(.system(size: 15, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white)
                .lineLimit(1)

            Spacer()

            Button {
                viewModel.removeMember(member)
            } label: {
                Image(systemName: "minus")
                    .font(.system(size: 16, weight: .black))
                    .foregroundStyle(.black)
                    .frame(width: 30, height: 30)
                    .background(Color(red: 0.969, green: 0.749, blue: 0.761))
                    .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
            }
            .buttonStyle(.plain)
            .accessibilityLabel("\(member.login)を通知対象から外す")
        }
    }

    //  GitHub member検索Sheet表示button
    private var addMemberButton: some View {
        Button {
            isMemberSearchPresented = true
        } label: {
            HStack(spacing: 8) {
                Image(systemName: "plus")
                    .font(.system(size: 16, weight: .black))

                Text("Add member")
                    .font(.system(size: 13, weight: .bold, design: .monospaced))
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

    //  通知コメント入力Section
    private var commentsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            //  Section title
            sectionTitle("■ Comments")

            //  通知コメント入力欄
            TextEditor(text: $viewModel.comment)
                .font(.system(size: 15, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white)
                .scrollContentBackground(.hidden)
                .frame(minHeight: 132)
                .padding(12)
                .background(AppTheme.fieldBackground)
                .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
                //  TextEditor placeholder
                .overlay(alignment: .topLeading) {
                    if viewModel.comment.isEmpty {
                        Text("今から実装タイムです。準備できたらpushしてね。")
                            .font(.system(size: 14, weight: .medium, design: .monospaced))
                            .foregroundStyle(.white.opacity(0.32))
                            .padding(.horizontal, 18)
                            .padding(.vertical, 20)
                            //  placeholderが入力操作を邪魔しないようにする
                            .allowsHitTesting(false)
                    }
                }
        }
    }

    //  下部固定エリア背景
    private var bottomBarBackground: some View {
        LinearGradient(
            colors: [AppTheme.background.opacity(0.70), AppTheme.background],
            startPoint: .top,
            endPoint: .bottom
        )
        .ignoresSafeArea(edges: .bottom)
    }

    //  Repo Settingと揃えたSection title
    private func sectionTitle(_ title: String) -> some View {
        Text(title)
            .font(.system(size: 20, weight: .regular, design: .monospaced))
            .foregroundStyle(Color(red: 0.929, green: 0.784, blue: 0.827))
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
