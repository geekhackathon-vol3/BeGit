import Foundation

enum NotificationChannelPlatform: String, Codable, CaseIterable, Sendable {
    case slack
    case discord

    var title: String { rawValue.capitalized }
    var symbolName: String { self == .slack ? "number" : "bubble.left.and.bubble.right.fill" }
    var webhookPlaceholder: String {
        self == .slack
            ? "https://hooks.slack.com/services/…"
            : "https://discord.com/api/webhooks/…"
    }
}

struct NotificationChannel: Identifiable, Equatable, Sendable {
    let id: Int64
    let platform: NotificationChannelPlatform
    let displayName: String
    let isEnabled: Bool
    let eventTypes: [String]
}

protocol NotificationChannelAPI: Sendable {
    func listNotificationChannels(repositoryID: Int64, accessToken: String) async throws -> [NotificationChannel]
    func createNotificationChannel(
        repositoryID: Int64,
        platform: NotificationChannelPlatform,
        displayName: String,
        webhookURL: String,
        accessToken: String
    ) async throws -> NotificationChannel
    func setNotificationChannelEnabled(repositoryID: Int64, channelID: Int64, enabled: Bool, accessToken: String) async throws -> NotificationChannel
    func testNotificationChannel(repositoryID: Int64, channelID: Int64, accessToken: String) async throws
    func deleteNotificationChannel(repositoryID: Int64, channelID: Int64, accessToken: String) async throws
}

struct BeGitTimeSchedule: Equatable, Sendable {
    let isEnabled: Bool
    let localTime: String
    let timezone: String
    let weekdays: [Int]
    let nextRunAt: Date?
}

protocol BeGitTimeScheduleAPI: Sendable {
    func getBeGitTimeSchedule(repositoryID: Int64, accessToken: String) async throws -> BeGitTimeSchedule
    func updateBeGitTimeSchedule(
        repositoryID: Int64,
        enabled: Bool,
        localTime: String,
        timezone: String,
        weekdays: [Int],
        accessToken: String
    ) async throws -> BeGitTimeSchedule
}
