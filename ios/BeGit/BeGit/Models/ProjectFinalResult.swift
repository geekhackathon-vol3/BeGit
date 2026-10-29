import Foundation

struct ProjectFinalResult: Decodable, Identifiable {
    let groupID: Int64
    let completedAt: String
    let members: [ProjectMemberPoints]
    let teamTotal: Int
    let modeResults: [ProjectModeResult]

    var id: Int64 { groupID }

    init(
        groupID: Int64,
        completedAt: String,
        members: [ProjectMemberPoints],
        teamTotal: Int,
        modeResults: [ProjectModeResult]
    ) {
        self.groupID = groupID
        self.completedAt = completedAt
        self.members = members
        self.teamTotal = teamTotal
        self.modeResults = modeResults
    }

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
    let pending: Int
    let bonusPoints: Int

    var id: BeGitGameMode { gameMode }

    enum CodingKeys: String, CodingKey {
        case gameMode = "game_mode", attempts, successes, failures, cancelled, pending, bonusPoints = "bonus_points"
    }

    init(
        gameMode: BeGitGameMode,
        attempts: Int,
        successes: Int,
        failures: Int,
        cancelled: Int,
        pending: Int = 0,
        bonusPoints: Int
    ) {
        self.gameMode = gameMode
        self.attempts = attempts
        self.successes = successes
        self.failures = failures
        self.cancelled = cancelled
        self.pending = pending
        self.bonusPoints = bonusPoints
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        gameMode = try container.decode(BeGitGameMode.self, forKey: .gameMode)
        attempts = try container.decode(Int.self, forKey: .attempts)
        successes = try container.decode(Int.self, forKey: .successes)
        failures = try container.decode(Int.self, forKey: .failures)
        cancelled = try container.decode(Int.self, forKey: .cancelled)
        pending = try container.decodeIfPresent(Int.self, forKey: .pending) ?? 0
        bonusPoints = try container.decode(Int.self, forKey: .bonusPoints)
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

    init(
        userID: Int64,
        login: String,
        avatarURL: String,
        reportPoints: Int,
        bonusPoints: Int,
        totalPoints: Int
    ) {
        self.userID = userID
        self.login = login
        self.avatarURL = avatarURL
        self.reportPoints = reportPoints
        self.bonusPoints = bonusPoints
        self.totalPoints = totalPoints
    }

    enum CodingKeys: String, CodingKey {
        case userID = "user_id", login, avatarURL = "avatar_url"
        case reportPoints = "report_points", bonusPoints = "bonus_points", totalPoints = "total_points"
    }
}

struct ProjectPointsSummary: Decodable, Identifiable {
    let groupID: Int64
    let isFinal: Bool
    let completedAt: String?
    let members: [ProjectMemberPoints]
    let teamTotal: Int
    let modeResults: [ProjectModeResult]

    var id: Int64 { groupID }

    var finalResult: ProjectFinalResult? {
        guard isFinal, let completedAt else { return nil }
        return ProjectFinalResult(
            groupID: groupID,
            completedAt: completedAt,
            members: members,
            teamTotal: teamTotal,
            modeResults: modeResults
        )
    }

    enum CodingKeys: String, CodingKey {
        case groupID = "group_id", isFinal = "is_final", completedAt = "completed_at"
        case members, teamTotal = "team_total", modeResults = "mode_results"
    }

    init(finalResult: ProjectFinalResult) {
        groupID = finalResult.groupID
        isFinal = true
        completedAt = finalResult.completedAt
        members = finalResult.members
        teamTotal = finalResult.teamTotal
        modeResults = finalResult.modeResults
    }
}
