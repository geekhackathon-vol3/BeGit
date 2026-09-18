//  MakeNotificationViewModel.swift
//  通知作成画面のローカル入力状態管理

import Foundation
import Combine

@MainActor
final class MakeNotificationViewModel: ObservableObject {
    let repository: Repository                      //  通知対象Repository

    @Published var members: [RepositoryMember]      //  Repository member一覧
    @Published private(set) var repositoryMemberCandidates: [RepositoryMember] // Repository由来のmember候補一覧
    @Published var selectedMemberIDs: Set<UUID>     //  選択中member ID一覧
    @Published var comment = ""                     //  通知コメント入力値
    @Published private(set) var activeBeGitTime: ActiveBeGitTime?
    @Published private(set) var isSending = false   //  通知送信中
    @Published private(set) var isLoadingMembers = false // member同期中
    @Published var errorMessage: String?            //  APIエラー表示
    //  進行中のBeGit Time（サーバー同期時に取得）。あれば送信不可
    @Published private(set) var activeChallenge: ActiveChallenge?

    private let repositoryAPI: any RepositoryAPI    // Repository関連API

    init(repository: Repository, repositoryAPI: any RepositoryAPI = BeGitBackendAPI()) {
        self.repository = repository
        self.members = repository.members                           //  初期member一覧
        self.repositoryMemberCandidates = repository.members         //  初期Repository member候補一覧
        self.selectedMemberIDs = Set(repository.members.map(\.id))  //  初期状態では全memberを選択
        self.activeChallenge = repository.activeChallenge
        self.repositoryAPI = repositoryAPI
    }

    //  管理者member
    var adminMember: RepositoryMember? {
        members.first
    }

    //  選択中member一覧
    var selectedMembers: [RepositoryMember] {
        members.filter { selectedMemberIDs.contains($0.id) }
    }

    //  通知送信可能か（BeGit Time 進行中は 409 になるので送らせない）
    var canSend: Bool {
        selectedMembers.isEmpty == false && isSending == false && activeChallenge == nil
    }

    //  進行中のBeGit Timeの説明文（送信不可の理由）
    var activeChallengeMessage: String? {
        guard let activeChallenge else { return nil }
        let issuer = activeChallenge.issuer.login.isEmpty ? "メンバー" : activeChallenge.issuer.login
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "ja_JP")
        formatter.dateFormat = "HH:mm"
        let endsAt = formatter.string(from: activeChallenge.endsAt)
        if activeChallenge.canEnd {
            return "あなたが発行したBeGit Timeが進行中です（\(endsAt) まで）。Dashboardから終了すると、すぐに再発行できます。"
        }
        return "\(issuer) が発行したBeGit Timeが進行中です（\(endsAt) まで）。終了後に発行できます。"
    }
    // MARK: - Actions
    //  member選択状態切り替え
    func toggleSelection(for member: RepositoryMember) {
        if selectedMemberIDs.contains(member.id) {
            //  選択解除
            selectedMemberIDs.remove(member.id)
        } else {
            //  選択追加
            selectedMemberIDs.insert(member.id)
        }
    }

    //  member削除
    func removeMember(_ member: RepositoryMember) {
        members.removeAll { $0.id == member.id }
        //  選択状態からも削除
        selectedMemberIDs.remove(member.id)
    }

    //  GitHub検索結果からmember追加
    func addMember(_ member: RepositoryMember) {
        guard members.contains(where: { $0.login.caseInsensitiveCompare(member.login) == .orderedSame }) == false else {
            return
        }

        members.append(member)
        selectedMemberIDs.insert(member.id)
    }

    //  通知モデル生成
    func makeNotification(backendID: Int64? = nil) -> RepositoryNotification {
        RepositoryNotification(
            backendID: backendID,
            repository: repository,
            //  選択中memberのみ通知対象
            selectedMembers: selectedMembers,
            //  前後空白を除去
            comment: comment.trimmingCharacters(in: .whitespacesAndNewlines)
        )
    }

    func loadMembers(accessToken: String?) async {
        guard isLoadingMembers == false else { return }
        guard let accessToken else { return }
        guard let backendID = repository.backendID else {
            members = repository.members
            repositoryMemberCandidates = repository.members
            selectedMemberIDs = Set(repository.members.map(\.id))
            return
        }

        isLoadingMembers = true
        errorMessage = nil
        defer { isLoadingMembers = false }

        do {
            let syncedRepository = try await repositoryAPI.getRepository(id: backendID, accessToken: accessToken)
            members = syncedRepository.members
            repositoryMemberCandidates = syncedRepository.members
            selectedMemberIDs = Set(syncedRepository.members.map(\.id))
            activeChallenge = syncedRepository.activeChallenge
        } catch {
            members = repository.members
            repositoryMemberCandidates = repository.members
            selectedMemberIDs = Set(repository.members.map(\.id))
        }
    }

    func loadActiveBeGitTime(accessToken: String?) async {
        guard let accessToken, let repositoryID = repository.backendID else {
            activeBeGitTime = nil
            return
        }

        do {
            let active = try await repositoryAPI.getActiveBeGitTime(
                repositoryID: repositoryID,
                accessToken: accessToken
            )
            activeBeGitTime = active ?? ActiveBeGitTimeStore.load(repositoryID: repositoryID)
            if activeBeGitTime == nil {
                ActiveBeGitTimeStore.remove(repositoryID: repositoryID)
            }
        } catch {
            // 新しいactive APIが未反映の環境では送信直後の互換キャッシュを使う。
            activeBeGitTime = ActiveBeGitTimeStore.load(repositoryID: repositoryID)
        }
    }

    func sendNotification(accessToken: String?, sentBy: Int64? = nil) async -> RepositoryNotification? {
        guard isSending == false else { return nil }
        isSending = true
        errorMessage = nil
        defer { isSending = false }

        guard NotificationDeliveryMode.current.usesLocalNotificationMock else {
            guard let accessToken else {
                errorMessage = "アクセストークンが見つかりません。"
                return nil
            }

            guard let backendID = repository.backendID else {
                errorMessage = "このRepositoryはバックエンドに同期されていません。"
                return nil
            }

            do {
                let notificationID = try await repositoryAPI.sendNotification(repositoryID: backendID, accessToken: accessToken)
                ActiveBeGitTimeStore.save(
                    repositoryID: backendID,
                    notificationID: notificationID,
                    sentBy: sentBy ?? 0,
                    expiresAt: Date().addingTimeInterval(60 * 60)
                )
                await loadActiveBeGitTime(accessToken: accessToken)
                return makeNotification(backendID: notificationID)
            } catch BeGitAPIError.requestFailed(statusCode: 409, message: _) {
                // チャレンジ進行中（発行から1時間以内）、または設定によりこのスプリントは送信済み。
                // 送信できていないので結果画面へは進めない
                errorMessage = "今は送信できません。チャレンジが進行中か、このスプリントは送信済みです。"
                return nil
            } catch {
                errorMessage = "通知の送信に失敗しました。"
                return nil
            }
        }

        let notification = makeNotification()
        if let repositoryID = repository.backendID {
            ActiveBeGitTimeStore.save(
                repositoryID: repositoryID,
                sentBy: sentBy ?? 0,
                expiresAt: notification.createdAt.addingTimeInterval(60 * 60)
            )
        }
        LocalNotificationScheduler.shared.scheduleNotification(for: notification)
        return notification
    }
}
