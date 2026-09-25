import Combine
import Foundation

@MainActor
final class NotificationChannelViewModel: ObservableObject {
    @Published private(set) var channels: [NotificationChannel] = []
    @Published private(set) var isLoading = false
    @Published private(set) var canManage = true
    @Published var errorMessage: String?
    @Published var successMessage: String?

    private let repositoryID: Int64?
    private let api: any NotificationChannelAPI

    init(repositoryID: Int64?, api: any NotificationChannelAPI = BeGitBackendAPI()) {
        self.repositoryID = repositoryID
        self.api = api
    }

    func load(accessToken: String?) async {
        guard let repositoryID, let accessToken, !accessToken.isEmpty else { return }
        isLoading = true
        defer { isLoading = false }
        do {
            channels = try await api.listNotificationChannels(repositoryID: repositoryID, accessToken: accessToken)
            canManage = true
        } catch let BeGitAPIError.requestFailed(statusCode, _) where statusCode == 403 {
            canManage = false
            channels = []
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func create(platform: NotificationChannelPlatform, displayName: String, webhookURL: String, accessToken: String?) async -> Bool {
        guard let repositoryID, let accessToken else { return false }
        do {
            let channel = try await api.createNotificationChannel(
                repositoryID: repositoryID,
                platform: platform,
                displayName: displayName,
                webhookURL: webhookURL,
                accessToken: accessToken
            )
            channels.append(channel)
            successMessage = "\(platform.title)と接続しました 🌱"
            return true
        } catch {
            errorMessage = error.localizedDescription
            return false
        }
    }

    func setEnabled(_ enabled: Bool, channel: NotificationChannel, accessToken: String?) async {
        guard let repositoryID, let accessToken else { return }
        do {
            let updated = try await api.setNotificationChannelEnabled(
                repositoryID: repositoryID,
                channelID: channel.id,
                enabled: enabled,
                accessToken: accessToken
            )
            replace(updated)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func sendTest(channel: NotificationChannel, accessToken: String?) async {
        guard let repositoryID, let accessToken else { return }
        do {
            try await api.testNotificationChannel(repositoryID: repositoryID, channelID: channel.id, accessToken: accessToken)
            successMessage = "テスト通知を送りました ✨"
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func delete(channel: NotificationChannel, accessToken: String?) async {
        guard let repositoryID, let accessToken else { return }
        do {
            try await api.deleteNotificationChannel(repositoryID: repositoryID, channelID: channel.id, accessToken: accessToken)
            channels.removeAll { $0.id == channel.id }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func replace(_ channel: NotificationChannel) {
        guard let index = channels.firstIndex(where: { $0.id == channel.id }) else { return }
        channels[index] = channel
    }
}
