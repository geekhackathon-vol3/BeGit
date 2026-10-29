import Foundation
import XCTest
@testable import BeGit

@MainActor
final class SecurityAttackTests: XCTestCase {
    private let installationKey = "savedGitHubAppInstallationID"

    func testOAuthCallbackRejectsWrongSchemeHostAndState() throws {
        let urls = [
            "https://oauth-callback?state=expected&code=stolen",
            "begit://evil.example?state=expected&code=stolen",
            "begit://oauth-callback?state=attacker&code=stolen"
        ]
        for rawURL in urls {
            let url = try XCTUnwrap(URL(string: rawURL))
            XCTAssertThrowsError(try OAuthCallbackParser.extractCode(from: url, expectedState: "expected"), rawURL)
        }
    }

    func testOAuthCallbackRejectsDuplicateParametersAndUnexpectedPath() throws {
        let urls = [
            "begit://oauth-callback?state=expected&state=attacker&code=abc",
            "begit://oauth-callback?state=expected&code=abc&code=evil",
            "begit://oauth-callback/other?state=expected&code=abc"
        ]
        for rawURL in urls {
            let url = try XCTUnwrap(URL(string: rawURL))
            XCTAssertThrowsError(try OAuthCallbackParser.extractCode(from: url, expectedState: "expected"), rawURL)
        }
    }

    func testOAuthCallbackAcceptsSingleMatchingStateAndCode() throws {
        let url = try XCTUnwrap(URL(string: "begit://oauth-callback?state=expected&code=abc"))
        XCTAssertEqual(try OAuthCallbackParser.extractCode(from: url, expectedState: "expected"), "abc")
    }

    func testForgedInstallationCallbackCannotChangeLoggedOutState() throws {
        let previousInstallation = UserDefaults.standard.object(forKey: installationKey)
        defer { restoreInstallation(previousInstallation) }
        UserDefaults.standard.removeObject(forKey: installationKey)
        let auth = AuthState(keychainManager: EmptyKeychain())
        XCTAssertFalse(auth.isLoggedIn)
        let url = try XCTUnwrap(URL(string: "begit://github-app-setup?installation_id=987654321"))

        XCTAssertFalse(auth.handleGitHubAppInstallationURL(url))
        XCTAssertNil(auth.githubAppInstallationID)
        XCTAssertNil(UserDefaults.standard.object(forKey: installationKey))
    }

    func testInstallationCallbackRejectsDuplicateIDAndUnexpectedPath() throws {
        let previousInstallation = UserDefaults.standard.object(forKey: installationKey)
        defer { restoreInstallation(previousInstallation) }
        UserDefaults.standard.removeObject(forKey: installationKey)
        let auth = AuthState(keychainManager: FixedKeychain(token: "test-token"))
        XCTAssertTrue(auth.isLoggedIn)
        let urls = [
            "begit://github-app-setup?installation_id=42&installation_id=99",
            "begit://github-app-setup/other?installation_id=42"
        ]
        for rawURL in urls {
            let url = try XCTUnwrap(URL(string: rawURL))
            XCTAssertFalse(auth.handleGitHubAppInstallationURL(url), rawURL)
            XCTAssertNil(auth.githubAppInstallationID)
        }
    }

    func testInstallationCallbackAcceptsValidLoggedInRedirect() throws {
        let previousInstallation = UserDefaults.standard.object(forKey: installationKey)
        defer { restoreInstallation(previousInstallation) }
        UserDefaults.standard.removeObject(forKey: installationKey)
        let auth = AuthState(keychainManager: FixedKeychain(token: "test-token"))
        let url = try XCTUnwrap(URL(string: "begit://github-app-setup?installation_id=42&setup_action=install&account_login=example"))

        XCTAssertTrue(auth.handleGitHubAppInstallationURL(url))
        XCTAssertEqual(auth.githubAppInstallationID, 42)
    }

    func testNotificationRejectsUnknownTypeMalformedAndOverflowingGroupID() {
        let payloads: [[AnyHashable: Any]] = [
            ["type": "unknown", "group_id": "1"],
            ["type": "reaction", "group_id": "not-a-number", "post_id": "1"],
            ["type": "reaction", "group_id": "999999999999999999999999", "post_id": "1"]
        ]
        for payload in payloads {
            XCTAssertNil(NotificationPayload(userInfo: payload))
        }
    }

    func testNotificationRejectsNonPositiveIDsBeforeRouting() {
        let payloads: [[AnyHashable: Any]] = [
            ["type": "reaction", "group_id": "-1", "post_id": "2"],
            ["type": "reaction", "group_id": "1", "post_id": "0"],
            ["type": "sprint_end", "group_id": "1", "sprint_id": "-2"]
        ]
        for payload in payloads {
            XCTAssertNil(NotificationPayload(userInfo: payload)?.route, "Unexpected route for \(payload)")
        }
    }

    func testNotificationAcceptsPositiveIDs() {
        let payload = NotificationPayload(userInfo: ["type": "reaction", "group_id": "1", "post_id": "2"])
        XCTAssertNotNil(payload?.route)
    }

    private func restoreInstallation(_ previous: Any?) {
        if let previous {
            UserDefaults.standard.set(previous, forKey: installationKey)
        } else {
            UserDefaults.standard.removeObject(forKey: installationKey)
        }
    }
}

private struct EmptyKeychain: KeychainManaging {
    func saveAccessToken(_ token: String) throws {}
    func readAccessToken() throws -> String? { nil }
    func deleteAccessToken() throws {}
}

private struct FixedKeychain: KeychainManaging {
    let token: String

    func saveAccessToken(_ token: String) throws {}
    func readAccessToken() throws -> String? { token }
    func deleteAccessToken() throws {}
}
