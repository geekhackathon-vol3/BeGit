//  RepositoryPhotoGridView.swift
//  リポジトリの投稿写真をタイル表示するギャラリー

import SwiftUI

// Timeline / Result の中でも切り替えて使える、toolbarを持たないギャラリー本体。
@MainActor
struct RepositoryPhotoGalleryContentView: View {
    let repository: Repository
    let activities: [RepositoryActivity]

    private var photoActivities: [RepositoryActivity] {
        activities
            .filter { activity in
                activity.mainPhotoURL != nil || activity.imageName != nil
            }
            .sorted { $0.date > $1.date }
    }

    var body: some View {
        ZStack {
            AppTheme.background
                .ignoresSafeArea()

            GeometryReader { proxy in
                VStack(spacing: 0) {
                    photoHeader
                        .padding(.horizontal, 20)
                        .padding(.top, 20)
                        .padding(.bottom, 18)

                    RepositoryPhotoGridContentView(
                        activities: photoActivities,
                        availableWidth: proxy.size.width
                    )
                }
            }
        }
    }

    private var photoHeader: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text("Photos")
                .appFont(.title)
                .foregroundStyle(AppTheme.Text.primary)
                .frame(maxWidth: .infinity, alignment: .leading)

            Text(repository.name)
                .appFont(.sectionHeader)
                .foregroundStyle(AppTheme.Text.low)
                .lineLimit(1)
        }
    }
}

private struct RepositoryPhotoGridContentView: View {
    let activities: [RepositoryActivity]
    let tileWidth: CGFloat
    let tileHeight: CGFloat
    let columns: [GridItem]

    private let columnCount = 4
    // 背面（外カメ）写真はアルバム風の縦長 3:4 タイルで表示する。
    private let tileAspectRatio: CGFloat = 3.0 / 4.0
    private let spacing: CGFloat = 3
    private let horizontalPadding: CGFloat = 3

    init(activities: [RepositoryActivity], availableWidth: CGFloat) {
        self.activities = activities

        let calculatedTileSize = max(
            1,
            (availableWidth
                - (horizontalPadding * 2)
                - (spacing * CGFloat(columnCount - 1))) / CGFloat(columnCount)
        )
        self.tileWidth = calculatedTileSize
        self.tileHeight = calculatedTileSize / tileAspectRatio
        self.columns = Array(
            repeating: GridItem(.fixed(calculatedTileSize), spacing: spacing),
            count: columnCount
        )
    }

    var body: some View {
        ScrollView {
            Group {
                if activities.isEmpty {
                    emptyState
                } else {
                    LazyVGrid(columns: columns, spacing: spacing) {
                        ForEach(Array(activities.enumerated()), id: \.element.id) { index, activity in
                            RepositoryPhotoGridTile(
                                activity: activity,
                                width: tileWidth,
                                height: tileHeight,
                                rank: index + 1
                            )
                        }
                    }
                }
            }
            .padding(.horizontal, horizontalPadding)
            .padding(.top, horizontalPadding)
        }
    }

    private var emptyState: some View {
        VStack(spacing: 12) {
            Image(systemName: "photo.on.rectangle.angled")
                .font(.system(size: 28, weight: .semibold))
                .foregroundStyle(AppTheme.Text.low)

            Text("写真の投稿はありません")
                .appFont(.body)
                .foregroundStyle(AppTheme.Text.low)
        }
        .frame(maxWidth: .infinity, minHeight: 260)
    }
}

private struct RepositoryPhotoGridTile: View {
    let activity: RepositoryActivity
    let width: CGFloat
    let height: CGFloat
    let rank: Int

    @State private var isSwapped = false

    // 小窓はタイル幅に依存しても、画像自身のレイアウトが不定にならないよう
    // 常に同じ縦長の矩形へ収める。
    private var frontWidth: CGFloat { width * 0.36 }
    // 内カメの小窓は縦長で表示する。
    private var frontHeight: CGFloat { width * 0.60 }

    var body: some View {
        ZStack(alignment: .topLeading) {
            // 背景画像にもタイルのサイズを先に与える。親の ZStack に
            // 画像の固有サイズを計算させると、小窓の配置幅が不定になる。
            displayedMainContent
                .frame(width: width, height: height)
                .clipped()

            if activity.frontPhotoURL != nil || activity.frontImageName != nil {
                displayedFrontContent
                    .frame(width: frontWidth, height: frontHeight)
                    .clipped()
                    .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: 6, style: .continuous)
                            .stroke(Color.black, lineWidth: 2)
                    )
                    .shadow(color: .black.opacity(0.45), radius: 4, x: 0, y: 2)
                    .padding(6)
                    .contentShape(Rectangle())
                    .onTapGesture {
                        withAnimation(.easeInOut(duration: 0.22)) {
                            isSwapped.toggle()
                        }
                    }
            }

            Text("\(rank)")
                .font(.system(size: max(13, min(18, width * 0.18)), weight: .regular, design: .rounded))
                .foregroundStyle(.white)
                .shadow(color: .black.opacity(0.65), radius: 3, x: 0, y: 1)
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)
                .allowsHitTesting(false)
        }
        .frame(width: width, height: height)
        .clipped()
        .clipShape(RoundedRectangle(cornerRadius: 4, style: .continuous))
        .accessibilityLabel("投稿写真")
    }

    @ViewBuilder
    private var displayedMainContent: some View {
        if isSwapped {
            frontPhotoContent
        } else {
            photoContent
        }
    }

    @ViewBuilder
    private var displayedFrontContent: some View {
        if isSwapped {
            photoContent
        } else {
            frontPhotoContent
        }
    }

    @ViewBuilder
    private var photoContent: some View {
        if let url = activity.mainPhotoURL {
            AsyncImage(url: url) { phase in
                switch phase {
                case .success(let image):
                    image
                        .resizable()
                        .scaledToFill()
                case .failure:
                    localPhoto
                case .empty:
                    photoPlaceholder
                @unknown default:
                    photoPlaceholder
                }
            }
        } else {
            localPhoto
        }
    }

    @ViewBuilder
    private var frontPhotoContent: some View {
        if let url = activity.frontPhotoURL {
            AsyncImage(url: url) { phase in
                switch phase {
                case .success(let image):
                    image
                        .resizable()
                        .scaledToFill()
                case .failure:
                    localFrontPhoto
                case .empty:
                    photoPlaceholder
                @unknown default:
                    photoPlaceholder
                }
            }
        } else {
            localFrontPhoto
        }
    }

    @ViewBuilder
    private var localPhoto: some View {
        if let imageName = activity.imageName, UIImage(named: imageName) != nil {
            Image(imageName)
                .resizable()
                .scaledToFill()
        } else {
            photoPlaceholder
        }
    }

    @ViewBuilder
    private var localFrontPhoto: some View {
        if let imageName = activity.frontImageName, UIImage(named: imageName) != nil {
            Image(imageName)
                .resizable()
                .scaledToFill()
        } else {
            photoPlaceholder
        }
    }

    private var photoPlaceholder: some View {
        ZStack {
            AppTheme.cardBackground
            Image(systemName: "photo")
                .font(.system(size: 22, weight: .semibold))
                .foregroundStyle(AppTheme.Text.low)
        }
    }
}
