import Foundation

enum AppRuntime {
    nonisolated static let isUnitTesting: Bool = {
        #if DEBUG
        let environment = ProcessInfo.processInfo.environment
        return environment["BEGIT_UNIT_TESTING"] == "1"
            || environment["XCTestConfigurationFilePath"] != nil
        #else
        return false
        #endif
    }()
}
