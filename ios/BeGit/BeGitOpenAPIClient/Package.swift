// swift-tools-version: 6.0
// BeGit のバックエンド API クライアントを openapi.yaml から生成する独立モジュール。
// アプリ本体は SWIFT_DEFAULT_ACTOR_ISOLATION = MainActor だが、生成コードは
// nonisolated 前提で OpenAPIRuntime から呼ばれる。ここを別モジュール（既定 nonisolated）に
// 切り出すことで、アプリの MainActor 既定を保ったまま生成コードを成立させる。
import PackageDescription

let package = Package(
    name: "BeGitOpenAPIClient",
    platforms: [
        .iOS(.v18),
        .macOS(.v15),
        .visionOS(.v2),
    ],
    products: [
        .library(name: "BeGitOpenAPIClient", targets: ["BeGitOpenAPIClient"]),
    ],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-generator", from: "1.0.0"),
        .package(url: "https://github.com/apple/swift-openapi-runtime", from: "1.0.0"),
    ],
    targets: [
        .target(
            name: "BeGitOpenAPIClient",
            dependencies: [
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
            ],
            plugins: [
                .plugin(name: "OpenAPIGenerator", package: "swift-openapi-generator"),
            ]
        ),
    ]
)
