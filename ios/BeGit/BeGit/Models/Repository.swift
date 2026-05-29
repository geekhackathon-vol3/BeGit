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
    private static func mockMembers() -> [RepositoryMember] {
        [
            RepositoryMember(
                login: "Riochin",
                avatarURL: URL(string: "https://avatars.githubusercontent.com/u/175614867?v=4")
            ),
            RepositoryMember(
                login: "s2108tomoka",
                avatarURL: URL(string: "https://avatars.githubusercontent.com/u/163800046?v=4")
            ),
            RepositoryMember(
                login: "palm7710",
                avatarURL: URL(string: "https://avatars.githubusercontent.com/u/168710387?v=4")
            ),
            RepositoryMember(
                login: "liruly",
                avatarURL: URL(string: "https://avatars.githubusercontent.com/u/141731612?v=4")
            )
        ]
    }

    nonisolated static let mockRepositories: [Repository] = [
        Repository(
            name: "apple/swift",
            memberCount: 4,
            members: Self.mockMembers()
        ),
        Repository(
            name: "begit/mobile",
            memberCount: 4,
            members: Self.mockMembers()
        ),
        Repository(
            name: "openai/swift-sdk",
            memberCount: 4,
            members: Self.mockMembers()
        )
    ]
}
