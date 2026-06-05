//  AddRepositoryView.swift
//  Repositoryをローカル状態に追加するSheet画面

import SwiftUI

@MainActor
struct AddRepositoryView: View {
    @Environment(\.dismiss) private var dismiss                 //  Sheetを閉じるためのdismiss action
    @EnvironmentObject private var authState: AuthState         //  API認証トークン
    @StateObject private var viewModel: AddRepositoryViewModel  //  画面状態を管理するViewModel
    @ObservedObject private var oauthManager = GitHubOAuthManager.shared
    @State private var isMemberSearchPresented = false          //  GitHub member検索Sheet表示状態

    let onAdd: (Repository) -> Void                             //  Repository追加完了時のcallback

    //  デフォルトViewModelで初期化
    init(existingRepositories: [Repository] = [], onAdd: @escaping (Repository) -> Void) {
        _viewModel = StateObject(wrappedValue: AddRepositoryViewModel(existingRepositories: existingRepositories))
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
                            .appFont(.title)
                            .foregroundStyle(AppTheme.Text.primary)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(.bottom, 2)

                        repositoryPreview

                        repositorySelectionSection  //  Repository選択
                        membersSection          //  Team member設定

                        if let errorMessage = viewModel.errorMessage {
                            Text(errorMessage)
                                .appFont(.label)
                                .foregroundStyle(AppTheme.softPink)
                                .lineSpacing(3)
                        }

                        //  Repository追加完了
                        PrimaryButton(viewModel.isSaving ? "追加中..." : "完了", systemImage: "checkmark", isEnabled: viewModel.canComplete) {
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

                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        oauthManager.startLogin()
                    } label: {
                        Image(systemName: "person.badge.key.fill")
                            .foregroundStyle(AppTheme.softPink)
                            .frame(minWidth: 44, minHeight: 44)
                    }
                    .accessibilityLabel("GitHubで認証")
                }
            }
        }
        .tint(AppTheme.accent)
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
        .task {
            await viewModel.loadRepositories()
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

    //  Repository入力状態preview
    private var repositoryPreview: some View {
        HStack(spacing: 12) {
                if viewModel.repositoryPreviewName == nil {
                Image(systemName: "shippingbox")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(AppTheme.Text.low)
            } else {
                Image("github_default_icon")
                    .resizable()
                    .scaledToFill()
                    .frame(width: 18, height: 18)
                    .clipShape(Circle())
            }

            Text(viewModel.repositoryPreviewName ?? "Repository not selected")
                .appFont(.label)
                .foregroundStyle(viewModel.repositoryPreviewName == nil ? AppTheme.Text.disabled : AppTheme.Text.regular)
                .lineLimit(1)

            Spacer()
        }
    }

    //  Repository選択Section
    private var repositorySelectionSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            sectionTitle(
                "■ GitHub Repository",
                size: 20,
                weight: .regular,
                color: Color(red: 0.980, green: 0.973, blue: 0.780)
            )

            repositoryPickerBox
        }
    }

    //  GitHub Repository候補リスト
    private var repositoryPickerBox: some View {
        VStack(alignment: .leading, spacing: 10) {
            repositorySearchBar

            if viewModel.isLoadingRepositories {
                repositoryLoadingRow
            } else if let errorMessage = viewModel.repositoryListErrorMessage {
                repositoryErrorState(errorMessage)
            } else if viewModel.displayedRepositories.isEmpty {
                repositoryEmptyState
            } else {
                VStack(spacing: 8) {
                    ForEach(viewModel.displayedRepositories) { repository in
                        repositoryCandidateRow(repository)
                    }

                    if viewModel.canShowMoreRepositories {
                        showMoreRepositoriesButton
                    }
                }
            }
        }
        .frame(maxWidth: .infinity, minHeight: 74, alignment: .leading)
        .padding(12)
        .background(AppTheme.repositoryCardBackground)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(Color(red: 0.310, green: 0.322, blue: 0.357), lineWidth: 2)
        )
    }

    //  Repository検索バー
    private var repositorySearchBar: some View {
        HStack(spacing: 10) {
            Image(systemName: "magnifyingglass")
                .font(.system(size: 14, weight: .black))
                .foregroundStyle(AppTheme.Text.disabled)
                .frame(width: 18, height: 18)

            TextField(
                "",
                text: Binding(
                    get: { viewModel.repositorySearchText },
                    set: { viewModel.updateRepositorySearchText($0) }
                ),
                prompt: Text("Search repositories").foregroundColor(.white.opacity(0.72))
            )
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .appFont(.body)
            .foregroundStyle(AppTheme.Text.primary)
            .tint(AppTheme.accent)

            if viewModel.repositorySearchText.isEmpty == false {
                Button {
                    viewModel.updateRepositorySearchText("")
                } label: {
                    Image(systemName: "xmark")
                        .font(.system(size: 11, weight: .black))
                        .foregroundStyle(.black)
                        .frame(width: 22, height: 22)
                        .background(AppTheme.Text.regular)
                        .clipShape(RoundedRectangle(cornerRadius: 5, style: .continuous))
                }
                .buttonStyle(.plain)
                .accessibilityLabel("検索文字をクリア")
            }
        }
        .frame(maxWidth: .infinity, minHeight: 40, alignment: .leading)
        .padding(.horizontal, 10)
        .background(AppTheme.fieldBackground)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(Color.white.opacity(0.08), lineWidth: 1)
        )
    }

    //  Repository読み込み中表示
    private var repositoryLoadingRow: some View {
        HStack(spacing: 12) {
            ProgressView()
                .tint(AppTheme.accent)

                Text("Loading repositories")
                .appFont(.body)
                .foregroundStyle(AppTheme.Text.regular)
        }
        .frame(maxWidth: .infinity, minHeight: 48, alignment: .leading)
    }

    //  Repository候補なし表示
    private var repositoryEmptyState: some View {
        Text("No repositories found")
            .appFont(.body)
            .foregroundStyle(AppTheme.Text.disabled)
            .frame(maxWidth: .infinity, minHeight: 48, alignment: .leading)
    }

    //  Repository候補の追加表示button
    private var showMoreRepositoriesButton: some View {
        Button(action: viewModel.showMoreRepositories) {
            HStack(spacing: 8) {
                Image(systemName: "chevron.down")
                    .font(.system(size: 13, weight: .black))

                Text("Show more")
                    .font(.system(size: 13, weight: .bold, design: .monospaced))
            }
            .foregroundStyle(.black)
            .frame(maxWidth: .infinity)
            .frame(height: 38)
            .background(AppTheme.accent)
            .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        }
        .buttonStyle(.plain)
        .accessibilityLabel("リポジトリ候補をさらに表示")
    }

    //  Repository取得エラー表示
    private func repositoryErrorState(_ message: String) -> some View {
        VStack(alignment: .leading, spacing: 10) {
                Text(message)
                .appFont(.label)
                .foregroundStyle(AppTheme.Text.high)
                .fixedSize(horizontal: false, vertical: true)

            Button {
                Task {
                    await viewModel.loadRepositories()
                }
            } label: {
                HStack(spacing: 8) {
                    Image(systemName: "arrow.clockwise")
                        .font(.system(size: 13, weight: .black))

                    Text("Retry")
                        .font(.system(size: 13, weight: .bold, design: .monospaced))
                }
                .foregroundStyle(.black)
                .padding(.horizontal, 14)
                .frame(height: 34)
                .background(AppTheme.accent)
                .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
            }
            .buttonStyle(.plain)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    //  Repository候補行
    private func repositoryCandidateRow(_ repository: GitHubRepository) -> some View {
        let isSelected = viewModel.selectedRepository?.id == repository.id
        let isAlreadyAdded = viewModel.isAlreadyAdded(repository)

        return Button {
            Task {
                await viewModel.selectRepository(repository)
            }
        } label: {
            HStack(spacing: 12) {
                repositoryAvatar(repository)

                VStack(alignment: .leading, spacing: 4) {
                    HStack(spacing: 7) {
                        HighlightedRepositoryNameText(
                            text: repository.fullName,
                            query: viewModel.repositorySearchText
                        )
                        .foregroundStyle(isAlreadyAdded ? AppTheme.Text.low : AppTheme.Text.primary)
                            .lineLimit(1)

                        if repository.isPrivate {
                            Image(systemName: "lock.fill")
                                .font(.system(size: 10, weight: .bold))
                                .foregroundStyle(AppTheme.Text.regular)
                        }

                        if isAlreadyAdded {
                            alreadyAddedBadge
                        }
                    }

                    if let description = repository.description, description.isEmpty == false {
                        Text(description)
                            .appFont(.caption)
                            .foregroundStyle(isAlreadyAdded ? AppTheme.Text.muted : AppTheme.Text.low)
                            .lineLimit(1)
                    }
                }

                Spacer(minLength: 8)

                if isSelected {
                    Image(systemName: "checkmark")
                        .font(.system(size: 15, weight: .black))
                        .foregroundStyle(.black)
                        .frame(width: 28, height: 28)
                        .background(Color(red: 0.725, green: 0.976, blue: 0.902))
                        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
                }
            }
            .padding(10)
            .background(isSelected ? AppTheme.accent.opacity(0.18) : (isAlreadyAdded ? Color.white.opacity(0.025) : Color.white.opacity(0.04)))
            .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .stroke(isSelected ? AppTheme.accent.opacity(0.8) : Color.white.opacity(0.06), lineWidth: 1)
            )
            .contentShape(Rectangle())
            .opacity(isAlreadyAdded ? 0.78 : 1)
        }
        .buttonStyle(.plain)
        .accessibilityLabel(isAlreadyAdded ? "\(repository.fullName)は追加済み" : "\(repository.fullName)を選択")
    }

    private var alreadyAddedBadge: some View {
        Text("追加済み")
            .font(.system(size: 10, weight: .black, design: .monospaced))
            .foregroundStyle(.black.opacity(0.78))
            .padding(.horizontal, 7)
            .frame(height: 20)
            .background(AppTheme.Text.regular)
            .clipShape(RoundedRectangle(cornerRadius: 5, style: .continuous))
    }

    //  Repository owner avatar
    private func repositoryAvatar(_ repository: GitHubRepository) -> some View {
        AsyncImage(url: repository.ownerAvatarURL) { phase in
            switch phase {
            case .success(let image):
                image
                    .resizable()
                    .scaledToFill()
            default:
                Image("github_default_icon")
                    .resizable()
                    .scaledToFill()
            }
        }
        .frame(width: 34, height: 34)
        .clipShape(Circle())
        .background(
            Circle()
                .fill(Color.black.opacity(0.20))
        )
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
            memberListSubheader("Selected members")

            if viewModel.isLoadingMembers {
                memberLoadingState
            } else if let errorMessage = viewModel.memberListErrorMessage {
                memberErrorState(errorMessage)
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

    //  member未選択表示
    private var emptyMemberState: some View {
        Text(viewModel.selectedRepository == nil ? "Select a repository" : "No collaborators found")
            .appFont(.body)
            .foregroundStyle(.white.opacity(0.42))
            .frame(maxWidth: .infinity, minHeight: 34, alignment: .leading)
    }

    //  member読み込み中表示
    private var memberLoadingState: some View {
        Text("Loading members")
            .appFont(.body)
            .foregroundStyle(.white.opacity(0.42))
            .frame(maxWidth: .infinity, minHeight: 34, alignment: .leading)
    }

    //  member取得エラー表示
    private func memberErrorState(_ message: String) -> some View {
        Text(message)
            .appFont(.caption)
            .foregroundStyle(.white.opacity(0.72))
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, minHeight: 34, alignment: .leading)
    }

    //  memberリスト内小見出し
    private func memberListSubheader(_ title: String) -> some View {
        Text(title)
            .appFont(.sectionHeader)
            .foregroundStyle(.white.opacity(0.54))
            .textCase(.uppercase)
    }

    //  選択済みmember行
    private func selectedMemberRow(_ member: RepositoryMember) -> some View {
        HStack(spacing: 12) {
            AvatarView(member: member, size: 34)

            Text(member.login)
                .appFont(.subheadline)                .foregroundStyle(.white)
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
            .accessibilityLabel("\(member.login)を削除")
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

    //  招待済みmember候補行
    private func invitedMemberRow(_ member: RepositoryMember) -> some View {
        Button {
            viewModel.addInvitedMember(member)
        } label: {
            HStack(spacing: 12) {
                AvatarView(member: member, size: 34)

                Text(member.login)
                    .appFont(.subheadline)
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
        Task {
            guard let repository = await viewModel.createRepository(accessToken: authState.accessToken) else { return }

            onAdd(repository)
            dismiss()
        }
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

//  Repository検索文字列に一致した部分だけ強調表示
private struct HighlightedRepositoryNameText: View {
    let text: String
    let query: String

    var body: some View {
        highlightedText
            .font(.system(size: 14, weight: .black, design: .monospaced))
    }

    private var highlightedText: Text {
        let trimmedQuery = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmedQuery.isEmpty == false else {
            return Text(text)
                .foregroundStyle(.white)
        }

        var result = Text("")
        var searchStart = text.startIndex

        while searchStart < text.endIndex,
              let range = text.range(
                of: trimmedQuery,
                options: [.caseInsensitive, .diacriticInsensitive],
                range: searchStart..<text.endIndex
              ) {
            if searchStart < range.lowerBound {
                result = result + Text(String(text[searchStart..<range.lowerBound]))
                    .foregroundStyle(.white)
            }

            result = result + Text(String(text[range]))
                .foregroundStyle(AppTheme.accent)

            searchStart = range.upperBound
        }

        if searchStart < text.endIndex {
            result = result + Text(String(text[searchStart..<text.endIndex]))
                .foregroundStyle(.white)
        }

        return result
    }
}
