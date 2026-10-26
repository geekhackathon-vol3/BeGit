//  RepoSettingView.swift
//  リポジトリ設定シート（リポジトリ情報・メンバー一覧）

import Combine
import SwiftUI

@MainActor
struct RepoSettingView: View {
    let repository: Repository
    @Environment(\.dismiss) private var dismiss
    @EnvironmentObject private var authState: AuthState
    @StateObject private var notificationChannels: NotificationChannelViewModel
    @StateObject private var beGitTimeSchedule: BeGitTimeScheduleViewModel
    @State private var isShowingChannelSetup = false
    @State private var isShowingTimePicker = false
    @State private var channelToDelete: NotificationChannel?
    @State private var members: [RepositoryMember]
    @State private var memberRoleError: String?
    @State private var canManage: Bool
    @State private var finalResult: ProjectFinalResult?
    @State private var finalResultLoadFailed = false
    @State private var showingFinalResult = false
    @State private var showingCompletionConfirmation = false
    @State private var completionError: String?
    @State private var isCompleting = false
    private let memberRoleAPI: any GroupMemberRoleAPI

    init(repository: Repository, memberRoleAPI: any GroupMemberRoleAPI = BeGitBackendAPI()) {
        self.repository = repository
        self.memberRoleAPI = memberRoleAPI
        _members = State(initialValue: repository.members)
        _memberRoleError = State(initialValue: nil)
        _canManage = State(initialValue: repository.canManage)
        _notificationChannels = StateObject(wrappedValue: NotificationChannelViewModel(repositoryID: repository.backendID))
        _beGitTimeSchedule = StateObject(wrappedValue: BeGitTimeScheduleViewModel(repositoryID: repository.backendID))
    }

    var body: some View {
        NavigationStack {
            ZStack {
                AppTheme.background.ignoresSafeArea()

                ScrollView {
                    settingsSections
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
                await beGitTimeSchedule.load(accessToken: authState.accessToken)
                await loadFinalResult()
            }
            .sheet(isPresented: $showingFinalResult) {
                if let finalResult {
                    NavigationStack {
                        ProjectPointsView(repository: repository, initialFinalResult: finalResult)
                    }
                    .environmentObject(authState)
                }
            }
            .confirmationDialog("この作品を完成しますか？集計が確定し、新しいBeGit Timeは発行できなくなります。", isPresented: $showingCompletionConfirmation) {
                Button("完成する") { Task { await completeRepository() } }
                Button("キャンセル", role: .cancel) { }
            }
            .alert("作品完成", isPresented: Binding(
                get: { completionError != nil },
                set: { if !$0 { completionError = nil } }
            )) {
                Button("OK", role: .cancel) { }
            } message: {
                Text(completionError ?? "")
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
            .sheet(isPresented: $isShowingTimePicker) {
                NavigationStack {
                    ZStack {
                        AppTheme.background.ignoresSafeArea()

                        DatePicker(
                            "開始時刻",
                            selection: $beGitTimeSchedule.selectedTime,
                            displayedComponents: .hourAndMinute
                        )
                        .labelsHidden()
                        .datePickerStyle(.wheel)
                        .tint(AppTheme.accent)
                        .environment(\.timeZone, beGitTimeSchedule.timeZone)
                        .environment(\.colorScheme, .dark)
                        .padding(.horizontal, 20)
                    }
                    .navigationTitle("開始時刻")
                    .navigationBarTitleDisplayMode(.inline)
                    .toolbar {
                        ToolbarItem(placement: .confirmationAction) {
                            Button("完了") {
                                isShowingTimePicker = false
                            }
                            .foregroundStyle(AppTheme.accent)
                        }
                    }
                    .toolbarBackground(AppTheme.background, for: .navigationBar)
                    .toolbarBackground(.visible, for: .navigationBar)
                }
                .presentationDetents([.height(320)])
                .presentationDragIndicator(.visible)
            }
            .alert("Notification Channels", isPresented: Binding(
                get: { notificationChannels.errorMessage != nil || notificationChannels.successMessage != nil },
                set: { if !$0 { notificationChannels.errorMessage = nil; notificationChannels.successMessage = nil } }
            )) {
                Button("OK", role: .cancel) { }
            } message: {
                Text(notificationChannels.errorMessage ?? notificationChannels.successMessage ?? "")
            }
            .alert("メンバー権限", isPresented: Binding(
                get: { memberRoleError != nil },
                set: { if !$0 { memberRoleError = nil } }
            )) {
                Button("OK", role: .cancel) { }
            } message: {
                Text(memberRoleError ?? "")
            }
            // メンバー権限アラートのOKを黒で表示する。
            .tint(memberRoleError == nil ? AppTheme.accent : .black)
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

    private var settingsSections: some View {
        VStack(alignment: .leading, spacing: 24) {
            Text("Settings")
                .font(.custom("Bitcount", size: 34))
                .foregroundStyle(AppTheme.Text.primary)
            settingSection(title: "■ GitHub Repository", color: AppTheme.sectionYellow) { repoInfoRow }
            completionSection
            settingSection(title: "■ Automatic BeGit Time", color: AppTheme.accent) { beGitTimeScheduleSection }
            settingSection(title: "■ Notification Channels", color: AppTheme.checkmarkGreen) { notificationChannelSection }
            settingSection(title: "■ Team Members", color: AppTheme.sectionPink) { teamMembersSection }
        }
        .padding(.horizontal, 20)
        .padding(.top, 24)
        .padding(.bottom, 40)
    }

    private var completionSection: some View {
        settingSection(title: "■ 作品完成", color: AppTheme.accent) {
            if finalResult != nil {
                Button("完成リザルトを見る") { showingFinalResult = true }
                    .foregroundStyle(AppTheme.accent)
            } else if finalResultLoadFailed, repository.isCompleted || repository.canManage {
                //  読み込めないまま「完成」を出すと、完成済みの作品にも押せてしまう
                Button("完成リザルトを再読み込み") { Task { await loadFinalResult() } }
                    .foregroundStyle(AppTheme.accent)
            } else if repository.isCompleted {
                Text("完成リザルトを読み込み中...")
                    .foregroundStyle(AppTheme.Text.medium)
            } else if repository.canManage, repository.ownerVerified, repository.backendID != nil {
                //  サーバーは確認済みの管理者本人だけに完成を許可する
                Button(isCompleting ? "集計中..." : "完成") { showingCompletionConfirmation = true }
                    .foregroundStyle(AppTheme.accent)
                    .disabled(isCompleting)
            } else {
                Text("確認済みの管理者が作品を完成できます")
                    .foregroundStyle(AppTheme.Text.medium)
            }
        }
    }

    private var teamMembersSection: some View {
        VStack(spacing: 0) {
            ForEach(Array(members.enumerated()), id: \.element.id) { index, member in
                memberRow(member)
                if index < members.count - 1 {
                    Divider().background(Color.white.opacity(0.08)).padding(.leading, 56)
                }
            }
        }
        .background(Color.white.opacity(0.05))
        .clipShape(RoundedRectangle(cornerRadius: 12))
    }

    private func loadFinalResult() async {
        guard let repositoryID = repository.backendID,
              let accessToken = authState.accessToken else { return }
        do {
            finalResult = try await BeGitBackendAPI().getFinalResult(repositoryID: repositoryID, accessToken: accessToken)
            finalResultLoadFailed = false
        } catch {
            finalResult = nil
            finalResultLoadFailed = true
        }
    }

    private func completeRepository() async {
        guard let repositoryID = repository.backendID,
              let accessToken = authState.accessToken else { return }
        isCompleting = true
        defer { isCompleting = false }
        do {
            finalResult = try await BeGitBackendAPI().completeRepository(repositoryID: repositoryID, accessToken: accessToken)
            showingFinalResult = true
        } catch {
            completionError = error.localizedDescription
        }
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

    @ViewBuilder
    private var beGitTimeScheduleSection: some View {
        if beGitTimeSchedule.isLoading {
            HStack { Spacer(); ProgressView().tint(AppTheme.accent); Spacer() }
                .padding(20)
        } else if !beGitTimeSchedule.canManage {
            Label("投稿可能なメンバーが自動開催を設定できます", systemImage: "lock.fill")
                .font(.system(size: 12, weight: .medium, design: .monospaced))
                .foregroundStyle(AppTheme.Text.medium)
                .padding(14)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(AppTheme.fieldBackground)
                .clipShape(RoundedRectangle(cornerRadius: 12))
        } else {
            VStack(alignment: .leading, spacing: 16) {
                Toggle("設定時刻に自動で開始", isOn: $beGitTimeSchedule.isEnabled)
                    .font(.system(size: 13, weight: .bold, design: .monospaced))
                    .foregroundStyle(.white)
                    .tint(AppTheme.checkmarkGreen)

                HStack {
                    Text("開始時刻")
                        .font(.system(size: 13, weight: .semibold, design: .monospaced))
                        .foregroundStyle(.white.opacity(beGitTimeSchedule.isEnabled ? 1 : 0.55))

                    Spacer()

                    Button {
                        isShowingTimePicker = true
                    } label: {
                        Text(beGitTimeSchedule.selectedTimeText)
                            .font(.system(size: 15, weight: .semibold, design: .monospaced))
                            .foregroundStyle(AppTheme.accent.opacity(beGitTimeSchedule.isEnabled ? 1 : 0.45))
                            .frame(width: 88, height: 42)
                            .background(AppTheme.accent.opacity(beGitTimeSchedule.isEnabled ? 0.14 : 0.07))
                            .clipShape(RoundedRectangle(cornerRadius: 10))
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("開始時刻")
                    .accessibilityValue(beGitTimeSchedule.selectedTimeText)
                }
                .disabled(!beGitTimeSchedule.isEnabled)

                HStack(spacing: 6) {
                    ForEach(BeGitTimeScheduleViewModel.weekdayOptions, id: \.value) { weekday in
                        Button {
                            beGitTimeSchedule.toggleWeekday(weekday.value)
                        } label: {
                            Text(weekday.label)
                                .font(.system(size: 12, weight: .bold, design: .monospaced))
                                .frame(maxWidth: .infinity)
                                .padding(.vertical, 9)
                                .foregroundStyle(beGitTimeSchedule.weekdays.contains(weekday.value) ? .black : AppTheme.Text.medium)
                                .background(beGitTimeSchedule.weekdays.contains(weekday.value) ? AppTheme.accent : Color.white.opacity(0.05))
                                .clipShape(RoundedRectangle(cornerRadius: 8))
                        }
                        .buttonStyle(.plain)
                        .disabled(!beGitTimeSchedule.isEnabled)
                    }
                }

                if let message = beGitTimeSchedule.message {
                    Text(message)
                        .font(.system(size: 11, weight: .medium, design: .monospaced))
                        .foregroundStyle(beGitTimeSchedule.hasError ? AppTheme.softPink : AppTheme.checkmarkGreen)
                }

                Button {
                    Task { await beGitTimeSchedule.save(accessToken: authState.accessToken) }
                } label: {
                    HStack {
                        if beGitTimeSchedule.isSaving { ProgressView().tint(.black) }
                        Text("自動開催設定を保存")
                    }
                    .font(.system(size: 13, weight: .bold, design: .monospaced))
                    .foregroundStyle(.black)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 13)
                    .background(AppTheme.accent)
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                }
                .buttonStyle(.plain)
                .disabled(!beGitTimeSchedule.hasLoaded || beGitTimeSchedule.isSaving || beGitTimeSchedule.weekdays.isEmpty)
            }
            .padding(14)
            .background(AppTheme.fieldBackground)
            .clipShape(RoundedRectangle(cornerRadius: 14))
        }
    }

    private func memberRow(_ member: RepositoryMember) -> some View {
        HStack(spacing: 12) {
            AvatarView(member: member, size: 36)
            Text(member.login)
                .font(.system(size: 13, weight: .semibold, design: .monospaced))
                .foregroundStyle(.white.opacity(0.85))
            Spacer()

            if canManage, member.role != .owner {
                Menu {
                    Button("投稿可能") { updateMemberRole(member, role: .poster) }
                    Button("表示専用") { updateMemberRole(member, role: .viewer) }
                } label: {
                    Label(member.role.displayName, systemImage: "chevron.up.chevron.down")
                        .font(.system(size: 11, weight: .bold, design: .monospaced))
                        .foregroundStyle(member.role == .poster ? AppTheme.accent : AppTheme.Text.medium)
                }
            } else {
                Text(member.role.displayName)
                    .font(.system(size: 11, weight: .bold, design: .monospaced))
                    .foregroundStyle(member.role == .owner ? AppTheme.softPink : AppTheme.Text.medium)
            }
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 12)
    }

    private func updateMemberRole(_ member: RepositoryMember, role: RepositoryRole) {
        guard member.role != role,
              let repositoryID = repository.backendID,
              let userID = member.backendUserID,
              let accessToken = authState.accessToken else { return }
        Task {
            do {
                try await memberRoleAPI.updateMemberRole(
                    repositoryID: repositoryID,
                    userID: userID,
                    role: role,
                    accessToken: accessToken
                )
                guard let index = members.firstIndex(where: { $0.id == member.id }) else { return }
                members[index] = RepositoryMember(
                    id: member.id,
                    backendUserID: member.backendUserID,
                    login: member.login,
                    avatarURL: member.avatarURL,
                    role: role
                )
            } catch {
                memberRoleError = error.localizedDescription
            }
        }
    }

    @ViewBuilder
    private var notificationChannelSection: some View {
        if notificationChannels.isLoading {
            HStack { Spacer(); ProgressView().tint(AppTheme.checkmarkGreen); Spacer() }
                .padding(20)
        } else if !notificationChannels.canManage {
            Label("投稿可能なメンバーが通知先を設定できます", systemImage: "lock.fill")
                .font(.system(size: 12, weight: .medium, design: .monospaced))
                .foregroundStyle(AppTheme.Text.medium)
                .padding(14)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(AppTheme.fieldBackground)
                .clipShape(RoundedRectangle(cornerRadius: 12))
        } else {
            VStack(spacing: 10) {
                ForEach(notificationChannels.channels) { channel in
                    SwipeToDeleteNotificationChannelRow(
                        channelName: channel.displayName,
                        onDelete: {
                            channelToDelete = channel
                        }
                    ) {
                        notificationChannelRow(channel)
                    }
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
            }
        }
    }

    private func notificationChannelRow(_ channel: NotificationChannel) -> some View {
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

// HOMEのRepository行と同じ、左スワイプで削除ボタンを表示する通知チャネル行。
private struct SwipeToDeleteNotificationChannelRow<Content: View>: View {
    let channelName: String
    let onDelete: () -> Void
    let content: Content

    @State private var offsetX: CGFloat = 0

    private let deleteRevealWidth: CGFloat = 86
    private let deleteButtonSize: CGFloat = 58
    private let rowCornerRadius: CGFloat = 14

    init(
        channelName: String,
        onDelete: @escaping () -> Void,
        @ViewBuilder content: () -> Content
    ) {
        self.channelName = channelName
        self.onDelete = onDelete
        self.content = content()
    }

    var body: some View {
        ZStack(alignment: .trailing) {
            deleteButton
                .padding(.trailing, 10)
                .scaleEffect(isDeleteVisible ? 1 : 0.62)
                .opacity(isDeleteVisible ? 1 : 0)
                .animation(.spring(response: 0.30, dampingFraction: 0.46), value: isDeleteVisible)

            content
                .contentShape(RoundedRectangle(cornerRadius: rowCornerRadius, style: .continuous))
                .offset(x: offsetX)
                .highPriorityGesture(swipeGesture)
                .onTapGesture {
                    guard isDeleteVisible else { return }
                    withAnimation(.spring(response: 0.32, dampingFraction: 0.56)) {
                        offsetX = 0
                    }
                }
                .animation(.spring(response: 0.34, dampingFraction: 0.62), value: offsetX)
        }
    }

    private var deleteButton: some View {
        Button {
            withAnimation(.spring(response: 0.24, dampingFraction: 0.54)) {
                offsetX = -deleteRevealWidth - 10
            }

            DispatchQueue.main.asyncAfter(deadline: .now() + 0.12) {
                withAnimation(.spring(response: 0.26, dampingFraction: 0.70)) {
                    onDelete()
                }
            }
        } label: {
            Image(systemName: "trash.fill")
                .font(.system(size: 22, weight: .black))
                .foregroundStyle(.white)
                .frame(width: deleteButtonSize, height: deleteButtonSize)
                .background(Color(.systemRed))
                .clipShape(Circle())
                .shadow(color: Color(.systemRed).opacity(0.35), radius: 10, x: 0, y: 4)
        }
        .buttonStyle(.plain)
        .accessibilityLabel("\(channelName)を削除")
    }

    private var swipeGesture: some Gesture {
        DragGesture(minimumDistance: 12, coordinateSpace: .local)
            .onChanged { value in
                let horizontalMovement = value.translation.width
                guard abs(horizontalMovement) > abs(value.translation.height) else { return }

                if horizontalMovement < 0 {
                    offsetX = max(horizontalMovement, -deleteRevealWidth)
                } else if isDeleteVisible {
                    offsetX = min(-deleteRevealWidth + horizontalMovement, 0)
                }
            }
            .onEnded { value in
                let shouldReveal = value.translation.width < -38 || value.predictedEndTranslation.width < -deleteRevealWidth

                withAnimation(.spring(response: 0.36, dampingFraction: 0.48)) {
                    offsetX = shouldReveal ? -deleteRevealWidth : 0
                }
            }
    }

    private var isDeleteVisible: Bool {
        offsetX < -12
    }
}

@MainActor
private final class BeGitTimeScheduleViewModel: ObservableObject {
    static let weekdayOptions: [(value: Int, label: String)] = [
        (1, "月"), (2, "火"), (3, "水"), (4, "木"), (5, "金"), (6, "土"), (7, "日")
    ]

    @Published var isEnabled = false
    @Published var selectedTime = Date()
    @Published var weekdays: Set<Int> = [1, 2, 3, 4, 5]
    @Published private(set) var timezoneIdentifier = TimeZone.current.identifier
    @Published private(set) var isLoading = false
    @Published private(set) var isSaving = false
    @Published private(set) var hasLoaded = false
    @Published private(set) var canManage = true
    @Published private(set) var message: String?
    @Published private(set) var hasError = false

    var timeZone: TimeZone { TimeZone(identifier: timezoneIdentifier) ?? .current }
    var selectedTimeText: String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "ja_JP")
        formatter.timeZone = timeZone
        formatter.dateFormat = "HH:mm"
        return formatter.string(from: selectedTime)
    }

    private let repositoryID: Int64?
    private let api: any BeGitTimeScheduleAPI

    init(repositoryID: Int64?, api: any BeGitTimeScheduleAPI = BeGitBackendAPI()) {
        self.repositoryID = repositoryID
        self.api = api
    }

    func load(accessToken: String?) async {
        hasLoaded = false
        guard let repositoryID, let accessToken, !accessToken.isEmpty else { return }
        isLoading = true
        defer { isLoading = false }
        do {
            let schedule = try await api.getBeGitTimeSchedule(repositoryID: repositoryID, accessToken: accessToken)
            guard apply(schedule) else {
                hasError = true
                message = "自動開催設定を読み込めませんでした。"
                return
            }
            canManage = true
            hasLoaded = true
            hasError = false
            message = nil
        } catch let BeGitAPIError.requestFailed(statusCode, _) where statusCode == 403 {
            canManage = false
        } catch {
            hasError = true
            message = error.localizedDescription
        }
    }

    func toggleWeekday(_ weekday: Int) {
        if weekdays.contains(weekday) {
            weekdays.remove(weekday)
        } else {
            weekdays.insert(weekday)
        }
        message = nil
    }

    func save(accessToken: String?) async {
        guard hasLoaded, let repositoryID, let accessToken, !weekdays.isEmpty else { return }
        isSaving = true
        message = nil
        defer { isSaving = false }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = "HH:mm"
        do {
            let updated = try await api.updateBeGitTimeSchedule(
                repositoryID: repositoryID,
                enabled: isEnabled,
                localTime: formatter.string(from: selectedTime),
                timezone: timezoneIdentifier,
                weekdays: weekdays.sorted(),
                accessToken: accessToken
            )
            guard apply(updated) else {
                hasLoaded = false
                hasError = true
                message = "保存した自動開催設定を反映できませんでした。"
                return
            }
            hasError = false
            message = updated.isEnabled ? "自動開催を設定しました 🌱" : "自動開催をオフにしました"
        } catch {
            hasError = true
            message = error.localizedDescription
        }
    }

    private func apply(_ schedule: BeGitTimeSchedule) -> Bool {
        let timeParts = schedule.localTime.split(separator: ":", omittingEmptySubsequences: false)
        guard timeParts.count == 2,
              let hour = Int(timeParts[0]), (0...23).contains(hour),
              let minute = Int(timeParts[1]), (0...59).contains(minute),
              let scheduleTimeZone = TimeZone(identifier: schedule.timezone),
              !schedule.weekdays.isEmpty,
              schedule.weekdays.allSatisfy({ (1...7).contains($0) }) else {
            return false
        }
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = scheduleTimeZone
        guard let selectedTime = calendar.date(bySettingHour: hour, minute: minute, second: 0, of: Date()) else {
            return false
        }
        isEnabled = schedule.isEnabled
        timezoneIdentifier = schedule.timezone
        weekdays = Set(schedule.weekdays)
        self.selectedTime = selectedTime
        return true
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
