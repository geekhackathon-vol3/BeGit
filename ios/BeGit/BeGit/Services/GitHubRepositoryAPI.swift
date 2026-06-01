//  GitHubRepositoryAPI.swift
//  GitHub Repository一覧取得API

import Foundation

//  GitHub Repository一覧取得APIのインターフェース
protocol GitHubRepositoryAPI {
    func listRepositories(accessToken: String) async throws -> [GitHubRepository]
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
