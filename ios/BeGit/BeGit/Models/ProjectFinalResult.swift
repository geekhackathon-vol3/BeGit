import Foundation

struct ProjectFinalResult: Decodable, Identifiable {
    let groupID: Int64
    let completedAt: String
    let members: [ProjectMemberPoints]
    let teamTotal: Int
    let modeResults: [ProjectModeResult]

    var id: Int64 { groupID }

    enum CodingKeys: String, CodingKey {
        case groupID = "group_id", completedAt = "completed_at", members, teamTotal = "team_total"
        case modeResults = "mode_results"
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        groupID = try container.decode(Int64.self, forKey: .groupID)
        completedAt = try container.decode(String.self, forKey: .completedAt)
        members = try container.decode([ProjectMemberPoints].self, forKey: .members)
        teamTotal = try container.decode(Int.self, forKey: .teamTotal)
        modeResults = try container.decodeIfPresent([ProjectModeResult].self, forKey: .modeResults) ?? []
    }
}

struct ProjectModeResult: Decodable, Identifiable {
    let gameMode: BeGitGameMode
    let attempts: Int
    let successes: Int
    let failures: Int
    let cancelled: Int
    let bonusPoints: Int

    var id: BeGitGameMode { gameMode }

    enum CodingKeys: String, CodingKey {
        case gameMode = "game_mode", attempts, successes, failures, cancelled, bonusPoints = "bonus_points"
    }
}

struct ProjectMemberPoints: Decodable, Identifiable {
    let userID: Int64
    let login: String
    let avatarURL: String
    let reportPoints: Int
    let bonusPoints: Int
    let totalPoints: Int

    var id: Int64 { userID }

    enum CodingKeys: String, CodingKey {
        case userID = "user_id", login, avatarURL = "avatar_url"
        case reportPoints = "report_points", bonusPoints = "bonus_points", totalPoints = "total_points"
    }
}
