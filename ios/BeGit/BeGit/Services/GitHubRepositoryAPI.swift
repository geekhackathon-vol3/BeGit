//  GitHubRepositoryAPI.swift
//  GitHub Repository一覧取得API

import Foundation

//  GitHub Repository一覧取得APIのインターフェース
protocol GitHubRepositoryAPI {
    func listRepositories(accessToken: String) async throws -> [GitHubRepository]
    func listRepositoryMembers(repoFullName: String, accessToken: String) async throws -> [RepositoryMember]
    func searchUsers(query: String, accessToken: String) async throws -> [RepositoryMember]
}

func shouldUseMockGitHubAPI(accessToken: String?) -> Bool {
    guard let accessToken else { return false }

    return accessToken.hasPrefix("mock_access_token_") || accessToken.hasPrefix("dev_")
}

//  GitHub REST APIを使うRepository一覧取得クライアント
struct GitHubRepositoryClient: GitHubRepositoryAPI {
    private let session: URLSession
    private let apiBaseURL = URL(string: "https://api.github.com")!

    init(session: URLSession = .shared) {
        self.session = session
    }

    func listRepositories(accessToken: String) async throws -> [GitHubRepository] {
        var components = URLComponents(url: apiBaseURL.appending(path: "user/repos"), resolvingAgainstBaseURL: false)
        components?.queryItems = [
            URLQueryItem(name: "affiliation", value: "owner,collaborator,organization_member"),
            URLQueryItem(name: "sort", value: "updated"),
            URLQueryItem(name: "direction", value: "desc"),
            URLQueryItem(name: "per_page", value: "100")
        ]

        guard let url = components?.url else {
            throw GitHubRepositoryAPIError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/vnd.github+json", forHTTPHeaderField: "Accept")
        request.setValue("2022-11-28", forHTTPHeaderField: "X-GitHub-Api-Version")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw GitHubRepositoryAPIError.invalidResponse
        }

        switch httpResponse.statusCode {
        case 200..<300:
            let decoder = JSONDecoder()
            decoder.dateDecodingStrategy = .iso8601
            return try decoder.decode([GitHubRepositoryResponse].self, from: data).map(\.repository)
        case 401:
            throw GitHubRepositoryAPIError.unauthorized
        default:
            throw GitHubRepositoryAPIError.requestFailed(statusCode: httpResponse.statusCode)
        }
    }

    func listRepositoryMembers(repoFullName: String, accessToken: String) async throws -> [RepositoryMember] {
        let components = repoFullName.split(separator: "/", maxSplits: 1).map(String.init)
        guard components.count == 2 else {
            throw GitHubRepositoryAPIError.invalidURL
        }

        var urlComponents = URLComponents(
            url: apiBaseURL
                .appending(path: "repos")
                .appending(path: components[0])
                .appending(path: components[1])
                .appending(path: "collaborators"),
            resolvingAgainstBaseURL: false
        )
        urlComponents?.queryItems = [
            URLQueryItem(name: "per_page", value: "100")
        ]

        guard let url = urlComponents?.url else {
            throw GitHubRepositoryAPIError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/vnd.github+json", forHTTPHeaderField: "Accept")
        request.setValue("2022-11-28", forHTTPHeaderField: "X-GitHub-Api-Version")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw GitHubRepositoryAPIError.invalidResponse
        }

        switch httpResponse.statusCode {
        case 200..<300:
            return try JSONDecoder().decode([GitHubCollaboratorResponse].self, from: data).map(\.member)
        case 401:
            throw GitHubRepositoryAPIError.unauthorized
        default:
            throw GitHubRepositoryAPIError.requestFailed(statusCode: httpResponse.statusCode)
        }
    }

    func searchUsers(query: String, accessToken: String) async throws -> [RepositoryMember] {
        let trimmedQuery = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmedQuery.isEmpty == false else {
            return []
        }

        var components = URLComponents(url: apiBaseURL.appending(path: "search/users"), resolvingAgainstBaseURL: false)
        components?.queryItems = [
            URLQueryItem(name: "q", value: "\(trimmedQuery) in:login"),
            URLQueryItem(name: "per_page", value: "20")
        ]

        guard let url = components?.url else {
            throw GitHubRepositoryAPIError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.setValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/vnd.github+json", forHTTPHeaderField: "Accept")
        request.setValue("2022-11-28", forHTTPHeaderField: "X-GitHub-Api-Version")

        let (data, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw GitHubRepositoryAPIError.invalidResponse
        }

        switch httpResponse.statusCode {
        case 200..<300:
            return try JSONDecoder().decode(GitHubUserSearchResponse.self, from: data).items.map(\.member)
        case 401:
            throw GitHubRepositoryAPIError.unauthorized
        default:
            throw GitHubRepositoryAPIError.requestFailed(statusCode: httpResponse.statusCode)
        }
    }
}

//  開発・Preview用のRepository一覧取得API
struct MockGitHubRepositoryAPI: GitHubRepositoryAPI {
    func listRepositories(accessToken: String) async throws -> [GitHubRepository] {
        try await Task.sleep(for: .milliseconds(350))

        return [
            GitHubRepository(
                id: 1,
                fullName: "apple/swift",
                description: "The Swift Programming Language",
                isPrivate: false,
                ownerAvatarURL: URL(string: "https://avatars.githubusercontent.com/u/10639145?v=4"),
                updatedAt: nil
            ),
            GitHubRepository(
                id: 2,
                fullName: "openai/swift-sdk",
                description: "Swift SDK for OpenAI APIs",
                isPrivate: false,
                ownerAvatarURL: URL(string: "https://avatars.githubusercontent.com/u/14957082?v=4"),
                updatedAt: nil
            ),
            GitHubRepository(
                id: 3,
                fullName: "begit/mobile",
                description: "BeGit iOS application",
                isPrivate: true,
                ownerAvatarURL: nil,
                updatedAt: nil
            )
        ]
    }

    func listRepositoryMembers(repoFullName: String, accessToken: String) async throws -> [RepositoryMember] {
        try await Task.sleep(for: .milliseconds(250))

        return [
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

    func searchUsers(query: String, accessToken: String) async throws -> [RepositoryMember] {
        try await Task.sleep(for: .milliseconds(250))

        let candidates = [
            RepositoryMember(
                login: "Palm7710",
                avatarURL: URL(string: "https://avatars.githubusercontent.com/u/168710387?v=4")
            ),
            RepositoryMember(
                login: "Riochin",
                avatarURL: URL(string: "https://avatars.githubusercontent.com/u/175614867?v=4")
            ),
            RepositoryMember(
                login: "s2108tomoka",
                avatarURL: URL(string: "https://avatars.githubusercontent.com/u/163800046?v=4")
            ),
            RepositoryMember(
                login: "liruly",
                avatarURL: URL(string: "https://avatars.githubusercontent.com/u/141731612?v=4")
            )
        ]

        let trimmedQuery = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard trimmedQuery.isEmpty == false else { return [] }

        return candidates.filter { $0.login.localizedCaseInsensitiveContains(trimmedQuery) }
    }
}

enum GitHubRepositoryAPIError: LocalizedError {
    case invalidURL
    case invalidResponse
    case unauthorized
    case requestFailed(statusCode: Int)

    var errorDescription: String? {
        switch self {
        case .invalidURL:
            return "GitHub Repository一覧URLの生成に失敗しました。"
        case .invalidResponse:
            return "GitHubから不正なレスポンスを受け取りました。"
        case .unauthorized:
            return "GitHub認証が無効です。再ログインしてください。"
        case let .requestFailed(statusCode):
            return "GitHub Repository一覧の取得に失敗しました。status=\(statusCode)"
        }
    }
}

private struct GitHubRepositoryResponse: Decodable {
    let id: Int
    let fullName: String
    let description: String?
    let isPrivate: Bool
    let owner: Owner
    let updatedAt: Date?

    var repository: GitHubRepository {
        GitHubRepository(
            id: id,
            fullName: fullName,
            description: description,
            isPrivate: isPrivate,
            ownerAvatarURL: owner.avatarURL,
            updatedAt: updatedAt
        )
    }

    private enum CodingKeys: String, CodingKey {
        case id
        case fullName = "full_name"
        case description
        case isPrivate = "private"
        case owner
        case updatedAt = "updated_at"
    }

    struct Owner: Decodable {
        let avatarURL: URL?

        private enum CodingKeys: String, CodingKey {
            case avatarURL = "avatar_url"
        }
    }
}

private struct GitHubCollaboratorResponse: Decodable {
    let id: Int64?
    let login: String
    let avatarURL: URL?

    var member: RepositoryMember {
        RepositoryMember(login: login, avatarURL: avatarURL)
    }

    private enum CodingKeys: String, CodingKey {
        case id
        case login
        case avatarURL = "avatar_url"
    }
}

private struct GitHubUserSearchResponse: Decodable {
    let items: [GitHubUserSearchItem]
}

private struct GitHubUserSearchItem: Decodable {
    let login: String
    let avatarURL: URL?

    var member: RepositoryMember {
        RepositoryMember(login: login, avatarURL: avatarURL)
    }

    private enum CodingKeys: String, CodingKey {
        case login
        case avatarURL = "avatar_url"
    }
}
