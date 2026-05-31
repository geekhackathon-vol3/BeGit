//  AddRepositoryViewModelTests.swift
//  Tests for AddRepositoryViewModel changes introduced in the repository preview / member selection PR

import XCTest
@testable import BeGit

@MainActor
final class AddRepositoryViewModelTests: XCTestCase {

    var viewModel: AddRepositoryViewModel!

    override func setUp() {
        super.setUp()
        viewModel = AddRepositoryViewModel()
    }

    override func tearDown() {
        viewModel = nil
        super.tearDown()
    }

    // MARK: - invitedMembers

    func test_invitedMembers_containsExpectedLogins() {
        let logins = viewModel.invitedMembers.map(\.login)
        XCTAssertEqual(logins, ["ayaka", "begit", "ios-dev", "repo-admin"])
    }

    func test_invitedMembers_countIsFour() {
        XCTAssertEqual(viewModel.invitedMembers.count, 4)
    }

    func test_invitedMembers_eachHasUniqueID() {
        let ids = viewModel.invitedMembers.map(\.id)
        let uniqueIDs = Set(ids)
        XCTAssertEqual(uniqueIDs.count, viewModel.invitedMembers.count)
    }

    // MARK: - repositoryPreviewName

    func test_repositoryPreviewName_isNilWhenURLTextIsEmpty() {
        viewModel.repositoryURLText = ""
        XCTAssertNil(viewModel.repositoryPreviewName)
    }

    func test_repositoryPreviewName_isNilForNonGitHubURL() {
        viewModel.repositoryURLText = "https://gitlab.com/apple/swift"
        XCTAssertNil(viewModel.repositoryPreviewName)
    }

    func test_repositoryPreviewName_isNilForMalformedURL() {
        viewModel.repositoryURLText = "not a url"
        XCTAssertNil(viewModel.repositoryPreviewName)
    }

    func test_repositoryPreviewName_returnsOwnerSlashRepoForValidGitHubURL() {
        viewModel.repositoryURLText = "https://github.com/apple/swift"
        XCTAssertEqual(viewModel.repositoryPreviewName, "apple/swift")
    }

    func test_repositoryPreviewName_isNilWhenURLHasOnlyOwner() {
        viewModel.repositoryURLText = "https://github.com/apple"
        XCTAssertNil(viewModel.repositoryPreviewName)
    }

    func test_repositoryPreviewName_stripsLeadingTrailingWhitespace() {
        viewModel.repositoryURLText = "  https://github.com/apple/swift  "
        XCTAssertEqual(viewModel.repositoryPreviewName, "apple/swift")
    }

    func test_repositoryPreviewName_returnsOwnerSlashRepoIgnoringDeepPath() {
        viewModel.repositoryURLText = "https://github.com/apple/swift/tree/main/lib"
        XCTAssertEqual(viewModel.repositoryPreviewName, "apple/swift")
    }

    // MARK: - selectableInvitedMembers

    func test_selectableInvitedMembers_returnsAllWhenNoMembersAdded() {
        XCTAssertEqual(viewModel.selectableInvitedMembers.count, 4)
    }

    func test_selectableInvitedMembers_excludesExactlyAddedMember() {
        let ayaka = viewModel.invitedMembers.first { $0.login == "ayaka" }!
        viewModel.addInvitedMember(ayaka)

        let selectableLogins = viewModel.selectableInvitedMembers.map(\.login)
        XCTAssertFalse(selectableLogins.contains("ayaka"))
        XCTAssertTrue(selectableLogins.contains("begit"))
        XCTAssertTrue(selectableLogins.contains("ios-dev"))
        XCTAssertTrue(selectableLogins.contains("repo-admin"))
    }

    func test_selectableInvitedMembers_isEmptyWhenAllInvitedMembersAreAdded() {
        for member in viewModel.invitedMembers {
            viewModel.addInvitedMember(member)
        }
        XCTAssertTrue(viewModel.selectableInvitedMembers.isEmpty)
    }

    func test_selectableInvitedMembers_excludesCaseInsensitiveDuplicate() {
        // A manually-added member whose login matches an invited member in different case
        // should be excluded from selectableInvitedMembers.
        viewModel.memberLoginText = "AYAKA"
        viewModel.addMember()

        let selectableLogins = viewModel.selectableInvitedMembers.map(\.login)
        XCTAssertFalse(selectableLogins.contains("ayaka"),
            "selectableInvitedMembers should exclude 'ayaka' when 'AYAKA' is already in members")
    }

    func test_selectableInvitedMembers_countDecreasesAfterEachAddedInvitedMember() {
        for (index, member) in viewModel.invitedMembers.enumerated() {
            XCTAssertEqual(viewModel.selectableInvitedMembers.count, 4 - index)
            viewModel.addInvitedMember(member)
        }
        XCTAssertEqual(viewModel.selectableInvitedMembers.count, 0)
    }

    // MARK: - showMemberInput (toggle behaviour)

    func test_showMemberInput_togglesFromFalseToTrue() {
        XCTAssertFalse(viewModel.isMemberInputVisible)
        viewModel.showMemberInput()
        XCTAssertTrue(viewModel.isMemberInputVisible)
    }

    func test_showMemberInput_togglesFromTrueToFalse() {
        viewModel.isMemberInputVisible = true
        viewModel.showMemberInput()
        XCTAssertFalse(viewModel.isMemberInputVisible)
    }

    func test_showMemberInput_togglesMultipleTimes() {
        XCTAssertFalse(viewModel.isMemberInputVisible)
        viewModel.showMemberInput()
        XCTAssertTrue(viewModel.isMemberInputVisible)
        viewModel.showMemberInput()
        XCTAssertFalse(viewModel.isMemberInputVisible)
        viewModel.showMemberInput()
        XCTAssertTrue(viewModel.isMemberInputVisible)
    }

    // MARK: - addInvitedMember

    func test_addInvitedMember_appendsMemberWhenNotPresent() {
        let member = RepositoryMember(login: "ayaka")
        viewModel.addInvitedMember(member)
        XCTAssertTrue(viewModel.members.contains { $0.login == "ayaka" })
    }

    func test_addInvitedMember_doesNotAddDuplicateByExactLogin() {
        let member = RepositoryMember(login: "ayaka")
        viewModel.addInvitedMember(member)
        viewModel.addInvitedMember(member)
        let count = viewModel.members.filter { $0.login == "ayaka" }.count
        XCTAssertEqual(count, 1)
    }

    func test_addInvitedMember_doesNotAddDuplicateCaseInsensitively() {
        let lower = RepositoryMember(login: "ayaka")
        let upper = RepositoryMember(login: "AYAKA")
        viewModel.addInvitedMember(lower)
        viewModel.addInvitedMember(upper)
        XCTAssertEqual(viewModel.members.count, 1)
    }

    func test_addInvitedMember_doesNotAddWhenSameMemberAlreadyAddedViaAddMember() {
        viewModel.memberLoginText = "begit"
        viewModel.addMember()

        let invitedBegit = viewModel.invitedMembers.first { $0.login == "begit" }!
        viewModel.addInvitedMember(invitedBegit)

        let count = viewModel.members.filter {
            $0.login.caseInsensitiveCompare("begit") == .orderedSame
        }.count
        XCTAssertEqual(count, 1)
    }

    func test_addInvitedMember_incrementsMembersCount() {
        XCTAssertEqual(viewModel.members.count, 0)
        viewModel.addInvitedMember(RepositoryMember(login: "ayaka"))
        XCTAssertEqual(viewModel.members.count, 1)
        viewModel.addInvitedMember(RepositoryMember(login: "begit"))
        XCTAssertEqual(viewModel.members.count, 2)
    }

    func test_addInvitedMember_preservesMemberObject() {
        let member = RepositoryMember(id: UUID(), login: "ios-dev", avatarURL: URL(string: "https://example.com/avatar.png"))
        viewModel.addInvitedMember(member)
        let added = viewModel.members.first { $0.login == "ios-dev" }
        XCTAssertNotNil(added)
        XCTAssertEqual(added?.id, member.id)
        XCTAssertEqual(added?.avatarURL, member.avatarURL)
    }

    // MARK: - Regression: addInvitedMember with mixed-case boundary

    func test_addInvitedMember_mixedCaseBoundary() {
        // All casing variants of the same login should be treated as duplicates
        viewModel.addInvitedMember(RepositoryMember(login: "Repo-Admin"))
        viewModel.addInvitedMember(RepositoryMember(login: "REPO-ADMIN"))
        viewModel.addInvitedMember(RepositoryMember(login: "repo-admin"))
        XCTAssertEqual(viewModel.members.count, 1)
    }
}
