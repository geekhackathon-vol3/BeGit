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
}
