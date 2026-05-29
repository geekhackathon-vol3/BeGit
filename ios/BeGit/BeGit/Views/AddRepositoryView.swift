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
                        headerSection           //  Header
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
                    //  Sheetを閉じる
                    Button("Cancel", action: dismiss.callAsFunction)
                        .font(.system(size: 14, weight: .bold, design: .monospaced))
                        .foregroundStyle(.white.opacity(0.72))
                }
            }
        }
        .tint(AppTheme.accent)
    }

    // MARK: - Components

    //  Header表示
    private var headerSection: some View {
        VStack(alignment: .center, spacing: 10) {
            Text("CONNECT A REPO")
                .font(.system(size: 13, weight: .black, design: .monospaced))
                .foregroundStyle(AppTheme.accent)

            Text("Paste a GitHub URL and add the teammates you want to track.")
                .font(.system(size: 15, weight: .medium, design: .monospaced))
                .foregroundStyle(.white.opacity(0.62))
                .multilineTextAlignment(.center)
                .lineSpacing(4)
        }
        .frame(maxWidth: .infinity)
    }

    //  Repository URL入力Section
    private var repositoryURLSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            sectionTitle("GitHub Repository URL")

            TextField("https://github.com/apple/swift", text: $viewModel.repositoryURLText)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .keyboardType(.URL)
                .font(.system(size: 15, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white)
                .padding(16)
                .background(AppTheme.fieldBackground)
                .clipShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
                .overlay(
                    RoundedRectangle(cornerRadius: 18, style: .continuous)
                        .stroke(Color.white.opacity(0.10), lineWidth: 1)
                )
        }
    }

    //  Team member設定Section
    private var membersSection: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                sectionTitle("Team Members")

                Spacer()

                Button(action: viewModel.showMemberInput) {
                    Image(systemName: "plus")
                        .font(.system(size: 14, weight: .black))
                        .foregroundStyle(.black)
                        .frame(width: 34, height: 34)
                        .background(AppTheme.accent)
                        .clipShape(Circle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("メンバー追加")
            }

            if viewModel.isMemberInputVisible {
                memberInput
            }

            if viewModel.members.isEmpty {
                Text("No members yet. Add GitHub login names locally for now.")
                    .font(.system(size: 13, weight: .medium, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.46))
                    .padding(.vertical, 8)
            } else {
                FlowLayout(spacing: 8) {
                    ForEach(viewModel.members) { member in
                        MemberChipView(member: member) {
                            viewModel.removeMember(member)
                        }
                    }
                }
            }
        }
    }

    //  member追加入力欄
    private var memberInput: some View {
        HStack(spacing: 10) {
            TextField("github-login", text: $viewModel.memberLoginText)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .font(.system(size: 15, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white)
                .padding(14)
                .background(AppTheme.fieldBackground)
                .clipShape(Capsule())

            Button(action: viewModel.addMember) {
                Image(systemName: "arrow.right")
                    .font(.system(size: 14, weight: .black))
                    .foregroundStyle(.black)
                    .frame(width: 46, height: 46)
                    .background(AppTheme.accent.opacity(viewModel.canAddMember ? 1 : 0.45))
                    .clipShape(Circle())
            }
            .buttonStyle(.plain)
            .disabled(viewModel.canAddMember == false)
        }
    }

    //  Section title共通View
    private func sectionTitle(_ title: String) -> some View {
        Text(title)
            .font(.system(size: 15, weight: .bold, design: .monospaced))
            .foregroundStyle(.white)
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
