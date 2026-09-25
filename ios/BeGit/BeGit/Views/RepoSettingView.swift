//  RepoSettingView.swift
//  リポジトリ設定シート（リポジトリ情報・メンバー一覧）

import SwiftUI

@MainActor
struct RepoSettingView: View {
    let repository: Repository
    @Environment(\.dismiss) private var dismiss
    @EnvironmentObject private var authState: AuthState
    @StateObject private var notificationChannels: NotificationChannelViewModel
    @State private var isShowingChannelSetup = false
    @State private var channelToDelete: NotificationChannel?

    init(repository: Repository) {
        self.repository = repository
        _notificationChannels = StateObject(wrappedValue: NotificationChannelViewModel(repositoryID: repository.backendID))
    }

    var body: some View {
        NavigationStack {
            ZStack {
                AppTheme.background.ignoresSafeArea()

                ScrollView {
                    VStack(alignment: .leading, spacing: 24) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Settings")
                                .font(.custom("Bitcount", size: 34))
                                .foregroundStyle(AppTheme.Text.primary)
                                .frame(maxWidth: .infinity, alignment: .leading)

                            Text(repository.name)
                                .font(.system(size: 12, weight: .semibold, design: .monospaced))
                                .foregroundStyle(AppTheme.Text.low)
                                .lineLimit(1)
                        }

                        // MARK: リポジトリ情報
                        settingSection(
                            title: "■ GitHub Repository",
                            color: AppTheme.sectionYellow
                        ) {
                            repoInfoRow
                        }

                        settingSection(
                            title: "■ Notification Channels",
                            color: AppTheme.checkmarkGreen
                        ) {
                            notificationChannelSection
                        }

                        // MARK: メンバー一覧
                        settingSection(
                            title: "■ Team Members",
                            color: AppTheme.sectionPink
                        ) {
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
            .task {
                await notificationChannels.load(accessToken: authState.accessToken)
            }
            .sheet(isPresented: $isShowingChannelSetup) {
                NotificationChannelSetupView { platform, name, webhookURL in
                    let created = await notificationChannels.create(
                        platform: platform,
                        displayName: name,
                        webhookURL: webhookURL,
                        accessToken: authState.accessToken
                    )
                    if created { isShowingChannelSetup = false }
                    return created
                }
            }
            .alert("Notification Channels", isPresented: Binding(
                get: { notificationChannels.errorMessage != nil || notificationChannels.successMessage != nil },
                set: { if !$0 { notificationChannels.errorMessage = nil; notificationChannels.successMessage = nil } }
            )) {
                Button("OK", role: .cancel) { }
            } message: {
                Text(notificationChannels.errorMessage ?? notificationChannels.successMessage ?? "")
            }
            .confirmationDialog(
                "\(channelToDelete?.displayName ?? "この通知先")との接続を解除しますか？",
                isPresented: Binding(
                    get: { channelToDelete != nil },
                    set: { if !$0 { channelToDelete = nil } }
                )
            ) {
                Button("接続を解除", role: .destructive) {
                    guard let channel = channelToDelete else { return }
                    channelToDelete = nil
                    Task { await notificationChannels.delete(channel: channel, accessToken: authState.accessToken) }
                }
                Button("キャンセル", role: .cancel) { channelToDelete = nil }
            }
        }
    }

    // MARK: - Components

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
    private var notificationChannelSection: some View {
        if notificationChannels.isLoading {
            HStack { Spacer(); ProgressView().tint(AppTheme.checkmarkGreen); Spacer() }
                .padding(20)
        } else if !notificationChannels.canManage {
            Label("リポジトリのオーナーだけが通知先を設定できます", systemImage: "lock.fill")
                .font(.system(size: 12, weight: .medium, design: .monospaced))
                .foregroundStyle(AppTheme.Text.medium)
                .padding(14)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(AppTheme.fieldBackground)
                .clipShape(RoundedRectangle(cornerRadius: 12))
        } else {
            VStack(spacing: 10) {
                ForEach(notificationChannels.channels) { channel in
                    notificationChannelRow(channel)
                }

                Button {
                    isShowingChannelSetup = true
                } label: {
                    Label(
                        notificationChannels.channels.isEmpty ? "Slack / Discordをつなぐ" : "通知先を追加",
                        systemImage: "plus.circle.fill"
                    )
                    .font(.system(size: 13, weight: .bold, design: .monospaced))
                    .foregroundStyle(Color.black)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 13)
                    .background(AppTheme.checkmarkGreen)
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                }
                .buttonStyle(.plain)

                Text("BeGit TimeとSprintのお知らせを、チームのいつもの場所へ。Webhook URLは暗号化して保存されます。")
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(AppTheme.Text.low)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private func notificationChannelRow(_ channel: NotificationChannel) -> some View {
        VStack(spacing: 12) {
            HStack(spacing: 12) {
                Image(systemName: channel.platform.symbolName)
                    .foregroundStyle(channel.platform == .slack ? AppTheme.softPink : AppTheme.accent)
                    .frame(width: 42, height: 42)

                VStack(alignment: .leading, spacing: 3) {
                    Text(channel.displayName)
                        .font(.system(size: 13, weight: .bold, design: .monospaced))
                        .foregroundStyle(.white)
                    Text(channel.platform.title)
                        .font(.system(size: 11, design: .monospaced))
                        .foregroundStyle(AppTheme.Text.medium)
                }
                Spacer()
                Toggle("", isOn: Binding(
                    get: { channel.isEnabled },
                    set: { enabled in
                        Task { await notificationChannels.setEnabled(enabled, channel: channel, accessToken: authState.accessToken) }
                    }
                ))
                .labelsHidden()
                .tint(AppTheme.checkmarkGreen)
            }

            HStack {
                Spacer()

                Button(role: .destructive) {
                    channelToDelete = channel
                } label: {
                    Image(systemName: "trash")
                }
                .buttonStyle(.borderless)
            }
            .font(.system(size: 12, weight: .semibold, design: .monospaced))
        }
        .padding(14)
        .background(AppTheme.fieldBackground)
        .clipShape(RoundedRectangle(cornerRadius: 14))
        .overlay {
            RoundedRectangle(cornerRadius: 14)
                .stroke(channel.isEnabled ? AppTheme.checkmarkGreen.opacity(0.24) : AppTheme.borderSubtle.opacity(0.45), lineWidth: 1)
        }
    }

    @ViewBuilder
    private func settingSection(
        title: String,
        color: Color,
        @ViewBuilder content: () -> some View
    ) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(title)
                .font(.system(size: 20, weight: .regular, design: .monospaced))
                .foregroundStyle(color)
            content()
        }
    }
}

// MARK: - Preview

struct RepoSettingView_Previews: PreviewProvider {
    static var previews: some View {
        RepoSettingView(repository: Repository.mockRepositories[0])
            .environmentObject(AuthState.shared)
            .previewDevice("iPhone 16 Pro Max")
    }
}

@MainActor
private struct NotificationChannelSetupView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var platform: NotificationChannelPlatform = .slack
    @State private var displayName = "BeGit"
    @State private var webhookURL = ""
    @State private var isSaving = false

    let onSave: (NotificationChannelPlatform, String, String) async -> Bool

    var body: some View {
        NavigationStack {
            ZStack {
                AppTheme.background.ignoresSafeArea()
                ScrollView {
                    VStack(alignment: .leading, spacing: 22) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Add Channel")
                                .font(.custom("Bitcount", size: 30))
                                .foregroundStyle(AppTheme.Text.primary)
                                .frame(maxWidth: .infinity, alignment: .leading)

                            Text("Incoming Webhookで接続")
                                .font(.system(size: 12, weight: .semibold, design: .monospaced))
                                .foregroundStyle(AppTheme.Text.low)
                        }

                        Picker("通知先", selection: $platform) {
                            ForEach(NotificationChannelPlatform.allCases, id: \.self) { item in
                                Text(item.title).tag(item)
                            }
                        }
                        .pickerStyle(.segmented)
                        .environment(\.colorScheme, .dark)

                        setupField("■ Display Name", text: $displayName, placeholder: "BeGit")

                        VStack(alignment: .leading, spacing: 8) {
                            Text("■ Webhook URL")
                                .font(.system(size: 20, weight: .regular, design: .monospaced))
                                .foregroundStyle(AppTheme.sectionYellow)
                            SecureField(
                                platform.webhookPlaceholder,
                                text: $webhookURL,
                                prompt: Text(platform.webhookPlaceholder).foregroundColor(AppTheme.Text.medium)
                            )
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .font(.system(size: 12, design: .monospaced))
                                .foregroundStyle(.white)
                                .padding(14)
                                .background(AppTheme.fieldBackground)
                                .clipShape(RoundedRectangle(cornerRadius: 12))
                        }

                        Button {
                            isSaving = true
                            Task {
                                _ = await onSave(platform, displayName, webhookURL)
                                isSaving = false
                            }
                        } label: {
                            HStack {
                                if isSaving { ProgressView().tint(.black) }
                                Text("接続する")
                            }
                            .font(.system(size: 14, weight: .bold, design: .monospaced))
                            .foregroundStyle(.black.opacity(0.82))
                            .frame(maxWidth: .infinity)
                            .padding(.vertical, 14)
                            .background(AppTheme.checkmarkGreen)
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                        }
                        .buttonStyle(.plain)
                        .disabled(displayName.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || webhookURL.isEmpty || isSaving)
                        .opacity(displayName.isEmpty || webhookURL.isEmpty ? 0.72 : 1)
                    }
                    .padding(20)
                }
            }
            .navigationTitle("")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button { dismiss() } label: {
                        Image(systemName: "xmark")
                            .font(.system(size: 14, weight: .semibold))
                    }
                    .foregroundStyle(AppTheme.softPink)
                    .accessibilityLabel("閉じる")
                }
            }
            .toolbarBackground(AppTheme.background, for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
        }
    }

    private func setupField(_ title: String, text: Binding<String>, placeholder: String) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.system(size: 20, weight: .regular, design: .monospaced))
                .foregroundStyle(AppTheme.sectionYellow)
            TextField(title, text: text, prompt: Text(placeholder).foregroundColor(AppTheme.Text.medium))
                .font(.system(size: 13, design: .monospaced))
                .foregroundStyle(.white)
                .padding(14)
                .background(AppTheme.fieldBackground)
                .clipShape(RoundedRectangle(cornerRadius: 12))
        }
    }
}
