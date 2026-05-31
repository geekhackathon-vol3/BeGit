//  AddRepositoryView.swift
//  Repositoryをローカル状態に追加するSheet画面

import SwiftUI

@MainActor
struct AddRepositoryView: View {
    @Environment(\.dismiss) private var dismiss                 //  Sheetを閉じるためのdismiss action
    @StateObject private var viewModel: AddRepositoryViewModel  //  画面状態を管理するViewModel

    let onAdd: (Repository) -> Void                             //  Repository追加完了時のcallback

    //  デフォルトViewModelで初期化
    init(onAdd: @escaping (Repository) -> Void) {
        _viewModel = StateObject(wrappedValue: AddRepositoryViewModel())
        self.onAdd = onAdd
    }

    //  外部ViewModel注入用
    init(viewModel: AddRepositoryViewModel, onAdd: @escaping (Repository) -> Void) {
        _viewModel = StateObject(wrappedValue: viewModel)
        self.onAdd = onAdd
    }

    var body: some View {
        //  Navigation対応Sheet
        NavigationStack {
            ZStack {
                //  背景色
                AppTheme.background
                    .ignoresSafeArea()

                ScrollView {
                    VStack(alignment: .leading, spacing: 24) {
                        Text("Repo Setting")
                            .font(.custom("Bitcount", size: 34))
                            .foregroundStyle(.white)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(.bottom, 2)

                        repositoryPreview

                        repositoryURLSection    //  Repository URL入力
                        membersSection          //  Team member設定

                        //  Repository追加完了
                        PrimaryButton("完了", systemImage: "checkmark", isEnabled: viewModel.canComplete) {
                            complete()
                        }
                        .padding(.top, 8)
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 26)
                    .padding(.bottom, 30)
                }
            }
            .navigationTitle("Add Repository")
            .navigationBarTitleDisplayMode(.inline)
            //  NavigationBar items
            .toolbar {
                ToolbarItem(placement: .principal) {
                    BeGitToolbarLogoView()
                }

                ToolbarItem(placement: .topBarLeading) {
                    BeGitBackButton()
                }
            }
        }
        .tint(AppTheme.accent)
    }

    // MARK: - Components

    //  Repository入力状態preview
    private var repositoryPreview: some View {
        HStack(spacing: 12) {
            if viewModel.repositoryPreviewName == nil {
                Image(systemName: "shippingbox")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(.white.opacity(0.50))
            } else {
                Image("github_default_icon")
                    .resizable()
                    .scaledToFill()
                    .frame(width: 18, height: 18)
                    .clipShape(Circle())
            }

            Text(viewModel.repositoryPreviewName ?? "Repository not selected")
                .font(.system(size: 13, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white.opacity(viewModel.repositoryPreviewName == nil ? 0.42 : 0.62))
                .lineLimit(1)

            Spacer()
        }
    }

    //  Repository URL入力Section
    private var repositoryURLSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            sectionTitle(
                "■ GitHub Repository URL",
                size: 20,
                weight: .regular,
                color: Color(red: 0.980, green: 0.973, blue: 0.780)
            )

            TextField("https://github.com/apple/swift", text: $viewModel.repositoryURLText)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .keyboardType(.URL)
                .font(.system(size: 15, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white)
                .padding(16)
                .background(Color(red: 0.247, green: 0.247, blue: 0.286))
                .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
                .overlay(
                    RoundedRectangle(cornerRadius: 6, style: .continuous)
                        .stroke(Color(red: 0.310, green: 0.322, blue: 0.357), lineWidth: 2)
                )
        }
    }

    //  Team member設定Section
    private var membersSection: some View {
        VStack(alignment: .leading, spacing: 14) {
            sectionTitle(
                "■ Team Members",
                size: 20,
                weight: .regular,
                color: Color(red: 0.929, green: 0.784, blue: 0.827)
            )

            memberListBox
        }
    }

    //  member選択リスト
    private var memberListBox: some View {
        VStack(alignment: .leading, spacing: 12) {
            memberListHeader

            if viewModel.members.isEmpty {
                emptyMemberState
            } else {
                VStack(spacing: 10) {
                    ForEach(viewModel.members) { member in
                        selectedMemberRow(member)
                    }
                }
            }

            if viewModel.isMemberInputVisible && viewModel.selectableInvitedMembers.isEmpty == false {
                Divider()
                    .background(Color.white.opacity(0.16))

                memberListSubheader("Invited members")

                VStack(spacing: 10) {
                    ForEach(viewModel.selectableInvitedMembers) { member in
                        invitedMemberRow(member)
                    }
                }
            }
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

    //  memberリスト内header
    private var memberListHeader: some View {
        HStack(spacing: 12) {
            memberListSubheader("Selected members")

            Spacer()

            Button(action: viewModel.showMemberInput) {
                Image(systemName: "plus")
                    .font(.system(size: 16, weight: .black))
                    .foregroundStyle(.black)
                    .frame(width: 34, height: 34)
                    .background(Color(red: 0.725, green: 0.976, blue: 0.902))
                    .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
            }
            .buttonStyle(.plain)
            .accessibilityLabel("メンバー追加")
        }
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
            .accessibilityLabel("\(member.login)を削除")
        }
    }

    //  招待済みmember候補行
    private func invitedMemberRow(_ member: RepositoryMember) -> some View {
        Button {
            viewModel.addInvitedMember(member)
        } label: {
            HStack(spacing: 12) {
                AvatarView(member: member, size: 34)

                Text(member.login)
                    .font(.system(size: 15, weight: .semibold, design: .monospaced))
                    .foregroundStyle(.white)
                    .lineLimit(1)

                Spacer()
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("\(member.login)を追加")
    }

    //  Section title共通View
    private func sectionTitle(
        _ title: String,
        size: CGFloat = 15,
        weight: Font.Weight = .bold,
        color: Color = .white
    ) -> some View {
        Text(title)
            .font(.system(size: size, weight: weight, design: .monospaced))
            .foregroundStyle(color)
    }

    //  Repository生成後に一覧へ追加
    private func complete() {
        guard let repository = viewModel.makeRepository() else { return }

        onAdd(repository)
        dismiss()
    }
}

//  Chip表示用の折り返しレイアウト
struct FlowLayout: Layout {
    let spacing: CGFloat    //  Chip表示間隔

    func sizeThatFits(
        proposal: ProposedViewSize,
        subviews: Subviews,
        cache: inout Void
    ) -> CGSize {
        let rows = rows(for: subviews, in: proposal.width ?? 0)
        return CGSize(
            width: proposal.width ?? rows.map(\.width).max() ?? 0,
            height: rows.reduce(0) { $0 + $1.height } + CGFloat(max(rows.count - 1, 0)) * spacing
        )
    }

    func placeSubviews(
        in bounds: CGRect,
        proposal: ProposedViewSize,
        subviews: Subviews,
        cache: inout Void
    ) {
        var origin = bounds.origin      //  描画開始位置

        for row in rows(for: subviews, in: bounds.width) {
            var xPosition = origin.x    //  現在行のX座標

            for element in row.elements {
                element.subview.place(
                    at: CGPoint(x: xPosition, y: origin.y),
                    proposal: ProposedViewSize(element.size)
                )
                xPosition += element.size.width + spacing
            }

            origin.y += row.height + spacing
        }
    }

    //  横幅に応じてsubviewを複数行へ分割
    private func rows(for subviews: Subviews, in width: CGFloat) -> [FlowRow] {
        var rows: [FlowRow] = []            //  完成済みrow一覧
        var currentRow = FlowRow()          //  現在構築中のrow
        let availableWidth = max(width, 1)  //  利用可能な横幅

        for subview in subviews {
            let size = subview.sizeThatFits(.unspecified)   //  subviewサイズ
            let nextWidth = currentRow.elements.isEmpty     //  次のsubviewを追加した場合の横幅
                ? size.width
                : currentRow.width + spacing + size.width

            //  rowを超える場合は次の行へ折り返し
            if nextWidth > availableWidth && currentRow.elements.isEmpty == false {
                rows.append(currentRow)
                currentRow = FlowRow()
            }

            currentRow.add(subview: subview, size: size, spacing: spacing)
        }

        if currentRow.elements.isEmpty == false {
            rows.append(currentRow)
        }

        return rows
    }
}

//  1行分のLayout情報
private struct FlowRow {
    var elements: [FlowElement] = []    //  Layout内のsubview一覧
    var width: CGFloat = 0              //  Row全体の横幅
    var height: CGFloat = 0             //  Row内で最も高いview高さ

    mutating func add(subview: LayoutSubview, size: CGSize, spacing: CGFloat) {
        if elements.isEmpty == false {
            width += spacing
        }

        elements.append(FlowElement(subview: subview, size: size))
        width += size.width
        height = max(height, size.height)
    }
}

//  Layout内のsubview情報
private struct FlowElement {
    let subview: LayoutSubview  //  配置対象のsubview
    let size: CGSize            //  subviewサイズ
}

struct AddRepositoryView_Previews: PreviewProvider {
    static var previews: some View {
        AddRepositoryView { _ in }
    }
}
