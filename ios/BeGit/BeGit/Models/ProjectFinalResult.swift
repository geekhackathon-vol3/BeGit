import Foundation

struct ProjectFinalResult: Decodable, Identifiable {
    let groupID: Int64
    let completedAt: String
    let members: [ProjectMemberPoints]
    let teamTotal: Int

    var id: Int64 { groupID }

    enum CodingKeys: String, CodingKey {
        case groupID = "group_id", completedAt = "completed_at", members, teamTotal = "team_total"
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
