//  BackendSchemaMapping.swift
//  OpenAPI 生成型(Components.Schemas.Handler_*) → ドメインモデル の変換。
//  生成型のプロパティは Optional・整数は Int なので、ここで既定値補完や Int64 変換を行う。

import Foundation

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
            date: createdAt.flatMap { ISO8601DateFormatter().date(from: $0) } ?? Date(),
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
            return "\(commits) commits in \(repoFullName ?? fallbackRepository.name)"
        }
        return status ?? "No activity yet"
    }
}
