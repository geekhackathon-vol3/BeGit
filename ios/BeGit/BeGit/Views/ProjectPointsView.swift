import SwiftUI
import UIKit

@MainActor
struct ProjectPointsView: View {
    @EnvironmentObject private var authState: AuthState
    @StateObject private var viewModel: ProjectPointsViewModel
    @State private var isShowingCopyToast = false

    init(repository: Repository, initialFinalResult: ProjectFinalResult? = nil) {
        _viewModel = StateObject(wrappedValue: ProjectPointsViewModel(
            repository: repository,
            initialFinalResult: initialFinalResult
        ))
    }

    init(viewModel: ProjectPointsViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        ZStack(alignment: .bottom) {
            AppTheme.background.ignoresSafeArea()

            ScrollView {
                LazyVStack(alignment: .leading, spacing: 20) {
                    header

                    if viewModel.isLoading {
                        ProgressView("Loading points...")
                            .tint(AppTheme.accent)
                            .foregroundStyle(AppTheme.Text.medium)
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 40)
                    } else if let summary = viewModel.summary {
                        statusSummary(summary)
                        teamTotalCard(summary.teamTotal)
                        memberSection(summary.members)
                        if summary.modeResults.isEmpty == false {
                            modeSection(summary.modeResults)
                        }
                        exportSection(summary)
                    }

                    if let errorMessage = viewModel.errorMessage {
                        Text(errorMessage)
                            .appFont(.label)
                            .foregroundStyle(AppTheme.softPink)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
                .padding(.horizontal, 20)
                .padding(.top, 20)
                .padding(.bottom, 40)
            }
            .refreshable {
                await viewModel.load(accessToken: authState.accessToken)
            }

            if isShowingCopyToast {
                copyToast
                    .padding(.horizontal, 20)
                    .padding(.bottom, 20)
                    .transition(.move(edge: .bottom).combined(with: .opacity))
            }
        }
        .navigationBarTitleDisplayMode(.inline)
        .toolbarBackground(AppTheme.background, for: .navigationBar)
        .toolbarBackground(.visible, for: .navigationBar)
        .toolbar {
            ToolbarItem(placement: .principal) { BeGitToolbarLogoView() }
        }
        .tint(AppTheme.accent)
        .task(id: authState.accessToken) {
            await viewModel.load(accessToken: authState.accessToken)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Points")
                .appFont(.title)
                .foregroundStyle(AppTheme.Text.primary)
            Text(viewModel.repository.name)
                .appFont(.sectionHeader)
                .foregroundStyle(AppTheme.Text.low)
                .lineLimit(1)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func statusSummary(_ summary: ProjectPointsSummary) -> some View {
        HStack(spacing: 12) {
            Label(
                summary.isFinal ? "FINAL RESULT" : "集計中",
                systemImage: summary.isFinal ? "trophy.fill" : "clock.arrow.circlepath"
            )
            .font(.system(size: 12, weight: .black, design: .monospaced))
            .foregroundStyle(summary.isFinal ? AppTheme.checkmarkGreen : AppTheme.accent)
            .padding(.horizontal, 12)
            .padding(.vertical, 7)
            .background((summary.isFinal ? AppTheme.checkmarkGreen : AppTheme.accent).opacity(0.12))
            .clipShape(Capsule())

            if let completedAt = summary.completedAt {
                Text(ProjectResultMarkdownFormatter().completionDateText(completedAt))
                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                    .foregroundStyle(AppTheme.Text.medium)
            }
        }
    }

    private func teamTotalCard(_ points: Int) -> some View {
        HStack(alignment: .firstTextBaseline) {
            Text("TEAM TOTAL")
                .font(.system(size: 15, weight: .black, design: .monospaced))
                .foregroundStyle(AppTheme.sectionPink)
            Spacer()
            Text("\(formatted(points)) pt")
                .font(.system(size: 25, weight: .black, design: .monospaced))
                .foregroundStyle(AppTheme.accent)
        }
        .padding(18)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay {
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(AppTheme.accent.opacity(0.75), lineWidth: 2)
        }
    }

    private func memberSection(_ members: [ProjectMemberPoints]) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            sectionTitle("■ MEMBER POINTS")
            ForEach(ProjectResultMarkdownFormatter().rankedMembers(members)) { ranked in
                memberCard(ranked.member, rank: ranked.rank)
            }
        }
    }

    private func memberCard(_ member: ProjectMemberPoints, rank: Int) -> some View {
        HStack(spacing: 12) {
            Text("#\(rank)")
                .font(.system(size: 13, weight: .black, design: .monospaced))
                .foregroundStyle(rank == 1 ? AppTheme.sectionYellow : AppTheme.Text.medium)
                .frame(width: 28)

            AvatarView(
                member: RepositoryMember(
                    backendUserID: member.userID,
                    login: member.login,
                    avatarURL: URL(string: member.avatarURL)
                ),
                size: 42
            )

            VStack(alignment: .leading, spacing: 5) {
                Text("@\(member.login)")
                    .font(.system(size: 15, weight: .bold, design: .monospaced))
                    .foregroundStyle(AppTheme.Text.primary)
                    .lineLimit(1)
                Text("報告 \(formatted(member.reportPoints)) pt  ·  ボーナス \(formatted(member.bonusPoints)) pt")
                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                    .foregroundStyle(AppTheme.Text.medium)
            }

            Spacer(minLength: 8)
            Text("\(formatted(member.totalPoints)) pt")
                .font(.system(size: 16, weight: .black, design: .monospaced))
                .foregroundStyle(AppTheme.accent)
        }
        .padding(14)
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
    }

    private func modeSection(_ modes: [ProjectModeResult]) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            sectionTitle("■ GAME RESULTS")
            ForEach(modes) { mode in
                HStack(spacing: 12) {
                    Image(systemName: mode.gameMode.systemImage)
                        .font(.system(size: 17, weight: .bold))
                        .foregroundStyle(AppTheme.accent)
                        .frame(width: 34, height: 34)
                        .background(AppTheme.accent.opacity(0.12))
                        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
                    VStack(alignment: .leading, spacing: 4) {
                        Text(mode.gameMode.title)
                            .appFont(.subheadline)
                            .foregroundStyle(AppTheme.Text.primary)
                        Text(modeResultText(mode))
                            .font(.system(size: 11, design: .monospaced))
                            .foregroundStyle(AppTheme.Text.medium)
                    }
                    Spacer()
                    Text("+\(formatted(mode.bonusPoints)) pt")
                        .font(.system(size: 13, weight: .bold, design: .monospaced))
                        .foregroundStyle(AppTheme.accent)
                }
                .padding(14)
                .background(AppTheme.fieldBackground)
                .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
            }
        }
    }

    private func exportSection(_ summary: ProjectPointsSummary) -> some View {
        let markdown = ProjectResultMarkdownFormatter().makeMarkdown(
            repositoryName: viewModel.repository.name,
            summary: summary
        )
        return VStack(alignment: .leading, spacing: 12) {
            sectionTitle("■ SHARE RESULT")
            if summary.finalResult == nil {
                Text("集計中の暫定リザルトとしてコピーされます")
                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                    .foregroundStyle(AppTheme.Text.medium)
            }
            PrimaryButton("Markdownをコピー", systemImage: "doc.on.doc.fill") {
                UIPasteboard.general.string = markdown
                showCopyToast()
            }

            ShareLink(item: markdown, subject: Text("\(viewModel.repository.name) Hackathon Result")) {
                HStack(spacing: 10) {
                    Image(systemName: "square.and.arrow.up")
                    Text("共有する")
                }
                .font(.system(size: 16, weight: .bold, design: .monospaced))
                .foregroundStyle(AppTheme.accent)
                .frame(maxWidth: .infinity)
                .frame(height: 56)
                .background(AppTheme.cardBackground)
                .clipShape(Capsule())
                .overlay(Capsule().stroke(AppTheme.accent, lineWidth: 2))
            }
        }
    }

    private var copyToast: some View {
        Label("クリップボードにコピーしました", systemImage: "checkmark.circle.fill")
            .font(.system(size: 13, weight: .bold, design: .monospaced))
            .foregroundStyle(.black)
            .padding(.horizontal, 16)
            .frame(height: 46)
            .background(AppTheme.checkmarkGreen)
            .clipShape(Capsule())
            .shadow(color: .black.opacity(0.35), radius: 12, y: 5)
    }

    private func sectionTitle(_ title: String) -> some View {
        Text(title)
            .font(.system(size: 18, weight: .regular, design: .monospaced))
            .foregroundStyle(AppTheme.sectionPink)
    }

    private func modeResultText(_ mode: ProjectModeResult) -> String {
        var parts = ["\(mode.successes)/\(mode.attempts) 達成"]
        if mode.cancelled > 0 { parts.append("途中終了 \(mode.cancelled)") }
        if mode.pending > 0 { parts.append("判定中 \(mode.pending)") }
        return parts.joined(separator: "  ·  ")
    }

    private func formatted(_ value: Int) -> String {
        value.formatted(.number.grouping(.automatic))
    }

    private func showCopyToast() {
        withAnimation(.spring(response: 0.30, dampingFraction: 0.82)) {
            isShowingCopyToast = true
        }
        Task {
            try? await Task.sleep(for: .seconds(2))
            withAnimation(.easeOut(duration: 0.20)) {
                isShowingCopyToast = false
            }
        }
    }
}

#Preview {
    NavigationStack {
        ProjectPointsView(repository: Repository.mockRepositories[0])
    }
    .environmentObject(AuthState.shared)
}
