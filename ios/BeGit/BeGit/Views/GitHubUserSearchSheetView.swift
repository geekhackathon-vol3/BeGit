//  GitHubUserSearchSheetView.swift
//  GitHubユーザー検索Sheet

import SwiftUI
import Combine

@MainActor
struct GitHubUserSearchSheetView: View {
    @Environment(\.dismiss) private var dismiss
    @StateObject private var viewModel: GitHubUserSearchViewModel

    private let existingMembers: [RepositoryMember]
    private let repositoryMembers: [RepositoryMember]
    private let onSelect: (RepositoryMember) -> Void

    init(
        accessToken: String?,
        existingMembers: [RepositoryMember],
        repositoryMembers: [RepositoryMember] = [],
        githubRepositoryAPI: (any GitHubRepositoryAPI)? = nil,
        onSelect: @escaping (RepositoryMember) -> Void
    ) {
        _viewModel = StateObject(
            wrappedValue: GitHubUserSearchViewModel(
                accessToken: accessToken,
                githubRepositoryAPI: githubRepositoryAPI
            )
        )
        self.existingMembers = existingMembers
        self.repositoryMembers = repositoryMembers
        self.onSelect = onSelect
    }

    var body: some View {
        NavigationStack {
            ZStack {
                AppTheme.background
                    .ignoresSafeArea()

                VStack(alignment: .leading, spacing: 16) {
                    Text("Add Member")
                        .appFont(.title)
                        .foregroundStyle(AppTheme.Text.primary)
                        .frame(maxWidth: .infinity, alignment: .leading)

                    searchBar
                    resultList

                    Spacer(minLength: 0)
                }
                .padding(.horizontal, 20)
                .padding(.top, 24)
                .padding(.bottom, 20)
            }
            .navigationTitle("Search GitHub User")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Close") {
                        dismiss()
                    }
                    .appFont(.body)
                    .foregroundStyle(AppTheme.accent)
                }

                ToolbarItem(placement: .principal) {
                    BeGitToolbarLogoView()
                }
            }
        }
        .tint(AppTheme.accent)
    }

    private var searchBar: some View {
        HStack(spacing: 10) {
            Image(systemName: "magnifyingglass")
                .font(.system(size: 14, weight: .black))
                .foregroundStyle(AppTheme.Text.disabled)
                .frame(width: 18, height: 18)

            TextField("Search GitHub username", text: $viewModel.searchText)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .appFont(.body)
                .foregroundStyle(AppTheme.Text.primary)
                .tint(AppTheme.accent)
                .submitLabel(.search)
                .onSubmit {
                    Task { await viewModel.search() }
                }

            if viewModel.searchText.isEmpty == false {
                Button {
                    viewModel.clear()
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

            Button {
                Task { await viewModel.search() }
            } label: {
                Image(systemName: "arrow.right")
                    .font(.system(size: 13, weight: .black))
                    .foregroundStyle(.black)
                    .frame(width: 30, height: 30)
                    .background(AppTheme.accent)
                    .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
            }
            .buttonStyle(.plain)
            .disabled(viewModel.canSearch == false)
            .opacity(viewModel.canSearch ? 1 : 0.45)
            .accessibilityLabel("GitHubユーザーを検索")
        }
        .frame(maxWidth: .infinity, minHeight: 44, alignment: .leading)
        .padding(.horizontal, 10)
        .background(AppTheme.fieldBackground)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(Color.white.opacity(0.08), lineWidth: 1)
        )
    }

    private var resultList: some View {
        VStack(alignment: .leading, spacing: 12) {
            if viewModel.isSearching {
                loadingState
            } else if let errorMessage = viewModel.errorMessage {
                errorState(errorMessage)
            } else if viewModel.hasSearched && visibleRepositoryMembers.isEmpty && visibleSearchResults.isEmpty {
                emptyState
            } else if viewModel.hasSearched == false && visibleRepositoryMembers.isEmpty {
                initialState
            } else {
                memberCandidateList
            }
        }
        .frame(maxWidth: .infinity, minHeight: 160, alignment: .topLeading)
        .padding(14)
        .background(AppTheme.cardBackground)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(AppTheme.borderSubtle, lineWidth: 2)
        )
    }

    private var loadingState: some View {
        HStack(spacing: 10) {
            ProgressView()
                .tint(AppTheme.accent)

            Text("Searching users")
                .appFont(.body)
                .foregroundStyle(AppTheme.Text.disabled)
        }
        .frame(maxWidth: .infinity, minHeight: 40, alignment: .leading)
    }

    private var emptyState: some View {
        Text("No users found")
            .appFont(.body)
            .foregroundStyle(AppTheme.Text.disabled)
            .frame(maxWidth: .infinity, minHeight: 40, alignment: .leading)
    }

    private var initialState: some View {
        Text("Search by GitHub username")
            .appFont(.body)
            .foregroundStyle(AppTheme.Text.disabled)
            .frame(maxWidth: .infinity, minHeight: 40, alignment: .leading)
    }

    private func errorState(_ message: String) -> some View {
        Text(message)
            .appFont(.label)
            .foregroundStyle(AppTheme.Text.high)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, minHeight: 40, alignment: .leading)
    }

    private var memberCandidateList: some View {
        VStack(alignment: .leading, spacing: 14) {
            if visibleRepositoryMembers.isEmpty == false {
                candidateSectionTitle("Repository users")

                VStack(spacing: 10) {
                    ForEach(visibleRepositoryMembers) { member in
                        userResultRow(member)
                    }
                }
            }

            if visibleSearchResults.isEmpty == false {
                candidateSectionTitle("GitHub search")

                VStack(spacing: 10) {
                    ForEach(visibleSearchResults) { member in
                        userResultRow(member)
                    }
                }
            }
        }
    }

    private func candidateSectionTitle(_ title: String) -> some View {
        Text(title)
            .appFont(.sectionHeader)
            .foregroundStyle(AppTheme.Text.muted)
            .textCase(.uppercase)
    }

    private func userResultRow(_ member: RepositoryMember) -> some View {
        let isAdded = containsExistingMember(member)

        return Button {
            guard isAdded == false else { return }
            onSelect(member)
            dismiss()
        } label: {
            HStack(spacing: 12) {
                AvatarView(member: member, size: 34)

                Text(member.login)
                    .appFont(.subheadline)
                    .foregroundStyle(AppTheme.Text.primary)
                    .lineLimit(1)

                Spacer()

                if isAdded {
                    Text("Added")
                        .font(.system(size: 12, weight: .black, design: .monospaced))
                        .foregroundStyle(AppTheme.Text.disabled)
                } else {
                    Image(systemName: "plus")
                        .font(.system(size: 16, weight: .black))
                        .foregroundStyle(.black)
                        .frame(width: 30, height: 30)
                        .background(AppTheme.accent)
                        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .disabled(isAdded)
        .accessibilityLabel(isAdded ? "\(member.login)は追加済み" : "\(member.login)を追加")
    }


    private func containsExistingMember(_ member: RepositoryMember) -> Bool {
        existingMembers.contains {
            $0.login.caseInsensitiveCompare(member.login) == .orderedSame
        }
    }

    private var visibleRepositoryMembers: [RepositoryMember] {
        repositoryMembers.filter { member in
            containsExistingMember(member) == false
                && matchesSearchText(member)
        }
    }

    private var visibleSearchResults: [RepositoryMember] {
        viewModel.results.filter { member in
            containsExistingMember(member) == false
                && visibleRepositoryMembers.contains {
                    $0.login.caseInsensitiveCompare(member.login) == .orderedSame
                } == false
        }
    }

    private func matchesSearchText(_ member: RepositoryMember) -> Bool {
        let query = viewModel.searchText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard query.isEmpty == false else { return true }

        return member.login.localizedCaseInsensitiveContains(query)
    }
}

@MainActor
final class GitHubUserSearchViewModel: ObservableObject {
    @Published var searchText = ""
    @Published private(set) var results: [RepositoryMember] = []
    @Published private(set) var isSearching = false
    @Published private(set) var hasSearched = false
    @Published var errorMessage: String?

    private let accessToken: String?
    private let githubRepositoryAPI: any GitHubRepositoryAPI

    init(accessToken: String?, githubRepositoryAPI: (any GitHubRepositoryAPI)? = nil) {
        self.accessToken = accessToken

        if let githubRepositoryAPI {
            self.githubRepositoryAPI = githubRepositoryAPI
        } else if shouldUseMockGitHubAPI(accessToken: accessToken) {
            self.githubRepositoryAPI = MockGitHubRepositoryAPI()
        } else {
            self.githubRepositoryAPI = GitHubRepositoryClient()
        }
    }

    var canSearch: Bool {
        searchText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty == false && isSearching == false
    }

    func search() async {
        guard canSearch else { return }
        guard let accessToken, accessToken.isEmpty == false else {
            errorMessage = "GitHubログイン情報を取得できませんでした。再ログインしてください。"
            return
        }

        isSearching = true
        hasSearched = true
        errorMessage = nil
        defer { isSearching = false }

        do {
            results = try await githubRepositoryAPI.searchUsers(query: searchText, accessToken: accessToken)
        } catch {
            errorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
        }
    }

    func clear() {
        searchText = ""
        results = []
        errorMessage = nil
        hasSearched = false
    }
}

struct GitHubUserSearchSheetView_Previews: PreviewProvider {
    static var previews: some View {
        GitHubUserSearchSheetView(
            accessToken: "mock_access_token_preview",
            existingMembers: [],
            onSelect: { _ in }
        )
    }
}
