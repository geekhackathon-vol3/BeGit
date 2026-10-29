import XCTest
@testable import BeGit

final class RepositoryActivityTests: XCTestCase {
    private let author = RepositoryMember(login: "tester", avatarURL: nil)

    func testGitHubActivityWithoutPhotosDoesNotHaveVisualMedia() {
        let activity = RepositoryActivity(
            type: .commit,
            title: "feat: add timeline",
            author: author
        )

        XCTAssertFalse(activity.hasVisualMedia)
    }

    func testActivityWithBackendPhotoHasVisualMedia() {
        let activity = RepositoryActivity(
            type: .memo,
            title: "写真で進捗報告",
            mainPhotoURL: URL(string: "https://example.com/main.jpg"),
            author: author
        )

        XCTAssertTrue(activity.hasVisualMedia)
    }

    func testMockActivityWithAssetHasVisualMedia() {
        let activity = RepositoryActivity(
            type: .commit,
            title: "Preview",
            imageName: "begit_mock_post_1",
            author: author
        )

        XCTAssertTrue(activity.hasVisualMedia)
    }

    func testBackendPostIdentityIsStableAcrossMappings() {
        let first = RepositoryActivity(
            backendPostID: 42,
            type: .commit,
            title: "first",
            author: author
        )
        let refreshed = RepositoryActivity(
            backendPostID: 42,
            type: .commit,
            title: "refreshed",
            author: author
        )

        XCTAssertEqual(first.id, refreshed.id)
    }

    func testRefreshMergePreservesClientStateAndSignedPhotoURL() throws {
        let originalURL = try XCTUnwrap(URL(string: "https://photos.example.com/posts/42.jpg?X-Amz-Date=old"))
        let refreshedURL = try XCTUnwrap(URL(string: "https://photos.example.com/posts/42.jpg?X-Amz-Date=new"))
        let reaction = ActivityReaction(type: .heart, count: 2, reactedByMe: true)
        let current = RepositoryActivity(
            backendPostID: 42,
            type: .commit,
            title: "before",
            mainPhotoURL: originalURL,
            author: author,
            reactions: [reaction]
        )
        let refreshed = RepositoryActivity(
            backendPostID: 42,
            type: .commit,
            title: "after",
            mainPhotoURL: refreshedURL,
            author: author
        )

        let merged = try XCTUnwrap(
            RepositoryActivity.mergingRefresh([refreshed], preserving: [current]).first
        )

        XCTAssertEqual(merged.id, current.id)
        XCTAssertEqual(merged.title, "after")
        XCTAssertEqual(merged.mainPhotoURL, originalURL)
        XCTAssertEqual(merged.reactions, [reaction])
    }

    func testRefreshMergeUsesURLForDifferentPhotoResource() throws {
        let current = RepositoryActivity(
            backendPostID: 42,
            type: .memo,
            title: "before",
            mainPhotoURL: URL(string: "https://photos.example.com/posts/old.jpg?signature=old"),
            author: author
        )
        let newURL = try XCTUnwrap(URL(string: "https://photos.example.com/posts/new.jpg?signature=new"))
        let refreshed = RepositoryActivity(
            backendPostID: 42,
            type: .memo,
            title: "after",
            mainPhotoURL: newURL,
            author: author
        )

        let merged = try XCTUnwrap(
            RepositoryActivity.mergingRefresh([refreshed], preserving: [current]).first
        )

        XCTAssertEqual(merged.mainPhotoURL, newURL)
    }
}
