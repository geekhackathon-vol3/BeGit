//  Repository.swift
//  BeGitで管理するGitHub Repositoryモデル

import Foundation

//  GitHub Repository情報
struct Repository: Identifiable, Equatable, Hashable, Sendable {
    let id: UUID                    //  Repository識別子
    let name: String                //  Repository名
    let memberCount: Int            //  Team member数
    let members: [RepositoryMember] //  Repository member一覧

    init(
        id: UUID = UUID(),
        name: String,
        memberCount: Int,
        members: [RepositoryMember]
    ) {
        self.id = id
        self.name = name
        self.memberCount = memberCount
        self.members = members
    }
}

//  Preview / Mock表示用データ
extension Repository {
    nonisolated static let mockRepositories: [Repository] = [
        Repository(
            name: "apple/swift",
            memberCount: 5,
            members: [
                RepositoryMember(login: "aya"),
                RepositoryMember(login: "ken"),
                RepositoryMember(login: "mika")
            ]
        ),
        Repository(
            name: "begit/mobile",
            memberCount: 3,
            members: [
                RepositoryMember(login: "palm"),
                RepositoryMember(login: "nora"),
                RepositoryMember(login: "dev")
            ]
        ),
        Repository(
            name: "openai/swift-sdk",
            memberCount: 8,
            members: [
                RepositoryMember(login: "codex"),
                RepositoryMember(login: "reviewer"),
                RepositoryMember(login: "ops")
            ]
        )
    ]
}
