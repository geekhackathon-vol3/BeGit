//  MakeNotificationView.swift
//  BeReal風Repository通知作成画面

import SwiftUI

@MainActor
struct MakeNotificationView: View {
    //  通知作成画面の状態を管理するViewModel
    @StateObject private var viewModel: MakeNotificationViewModel
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
                        //  共通Header
                        BeGitHeaderView(title: "make notification", subtitle: viewModel.repository.name)

                        //  通知対象member選択
                        membersSection
                        //  通知コメント入力
                        commentsSection
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 20)
                    .padding(.bottom, 104)  //  下部固定button領域分の余白  
                }

                //  通知を生成して結果画面へ遷移
                Button(action: sendNotification) {
                    PrimaryCapsuleButtonLabel(
                        title: "通知を送る",
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
    }

    // MARK: - Components

    //  Team member選択Section
    private var membersSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                //  Section title
                SectionTitleView("Team Members", caption: "select notification targets")

                Spacer()

                //  Mock member追加button
                Button(action: viewModel.addMockMember) {
                    Image(systemName: "plus")
                        .font(.system(size: 14, weight: .black))
                        .foregroundStyle(.black)
                        .frame(width: 36, height: 36)
                        .background(AppTheme.accent)
                        .clipShape(Circle())
                }
                .buttonStyle(.plain)
            }

            //  member選択一覧
            ForEach(viewModel.members) { member in
                MemberSelectionRowView(
                    member: member,
                    isSelected: viewModel.selectedMemberIDs.contains(member.id),
                    isAdmin: member.id == viewModel.adminMember?.id,
                    onToggle: {
                        viewModel.toggleSelection(for: member)
                    },
                    onRemove: {
                        viewModel.removeMember(member)
                    }
                )
            }
        }
    }

    //  通知コメント入力Section
    private var commentsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            //  Section title
            SectionTitleView("Comments", caption: "shown inside the notification")

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

    private func sendNotification() {
        onSend(viewModel.makeNotification())
    }
}

struct MakeNotificationView_Previews: PreviewProvider {
    static var previews: some View {
        NavigationStack {
            MakeNotificationView(repository: Repository.mockRepositories[0])
        }
        .previewDevice("iPhone SE (3rd generation)")
    }
}
