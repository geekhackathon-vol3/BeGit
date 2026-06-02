//  BackendSchemaMapping.swift
//  OpenAPI 生成型(Components.Schemas.Handler_*) → ドメインモデル の変換。
//  生成型のプロパティは Optional・整数は Int なので、ここで既定値補完や Int64 変換を行う。

import Foundation
import BeGitOpenAPIClient

// 共有 ISO8601 フォーマッタ：小数秒あり・なし両対応
private let sharedISO8601DateFormatter: ISO8601DateFormatter = {
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    return formatter
}()

extension Components.Schemas.Handler_UserJSON {
    func toGitHubUser() -> GitHubUser {
        GitHubUser(
            id: id ?? 0,
            login: login ?? "",
            avatarURL: avatarUrl.flatMap { URL(string: $0) },
            email: nil
        )
    }
}

extension Components.Schemas.Handler_GroupJSON {
    func toRepository(members: [RepositoryMember]) -> Repository {
        let fullName = repoFullName ?? ""
        let displayName = fullName.isEmpty ? (name ?? "") : fullName
        return Repository(
            backendID: id.map(Int64.init),
            name: displayName,
            memberCount: members.count,
            members: members
        )
    }
}

extension Components.Schemas.Handler_GroupDetailJSON {
    func toRepository() -> Repository {
        let repositoryMembers = (members ?? []).map { $0.toMember() }
        let fullName = repoFullName ?? ""
        let displayName = fullName.isEmpty ? (name ?? "") : fullName
        return Repository(
            backendID: id.map(Int64.init),
            name: displayName,
            memberCount: repositoryMembers.count,
            members: repositoryMembers
        )
    }
}

extension Components.Schemas.Handler_GroupMemberJSON {
    func toMember() -> RepositoryMember {
        RepositoryMember(
            backendUserID: userId.map(Int64.init),
            login: login ?? "",
            avatarURL: avatarUrl.flatMap { URL(string: $0) }
        )
    }
}

extension Components.Schemas.Handler_PostFeedJSON {
    func toActivity(fallbackRepository: Repository) -> RepositoryActivity {
        RepositoryActivity(
            type: activityType,
            title: activityTitle(fallbackRepository: fallbackRepository),
            date: createdAt.flatMap { sharedISO8601DateFormatter.date(from: $0) } ?? Date(),
            imageName: "begit_timeline_mock",
            author: RepositoryMember(
                backendUserID: userId.map(Int64.init),
                login: login ?? "",
                avatarURL: avatarUrl.flatMap { URL(string: $0) }
            ),
            reaction: reaction
        )
    }

    private var activityType: RepositoryActivityType {
        switch postType {
        case "pull_request", "pullRequest":
            return .pullRequest
        // "memo" が正。"sorry"/"comment" は旧名称・旧データ互換のため受理。
        case "memo", "sorry", "comment":
            return .memo
        default:
            return .commit
        }
    }

    private var reaction: RepositoryReaction? {
        switch activityType {
        case .commit:
            return .check
        case .pullRequest:
            return .heart
        case .memo:
            return .sorry
        }
    }

    private func activityTitle(fallbackRepository: Repository) -> String {
        if let latestCommitMessage, latestCommitMessage.isEmpty == false {
            return latestCommitMessage
        }
        if let body, body.isEmpty == false {
            return body
        }
        let commits = commitCount ?? 0
        if commits > 0 {
            let displayRepoName = (repoFullName?.isEmpty ?? true) ? fallbackRepository.name : repoFullName!
            return "\(commits) commits in \(displayRepoName)"
        }
        return status ?? "No activity yet"
    }
}
