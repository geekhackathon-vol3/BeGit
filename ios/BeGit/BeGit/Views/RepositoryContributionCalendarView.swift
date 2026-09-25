//  RepositoryContributionCalendarView.swift
//  リポジトリ内の全メンバーの進捗投稿を月間カレンダーで可視化する。

import SwiftUI

enum RepositoryDashboardDisplayMode: Hashable {
    case timeline
    case photos
    case calendar

    var next: RepositoryDashboardDisplayMode {
        switch self {
        case .timeline: .photos
        case .photos: .calendar
        case .calendar: .timeline
        }
    }

    var nextSystemImage: String {
        switch next {
        case .timeline: "list.bullet"
        case .photos: "square.grid.3x3.fill"
        case .calendar: "calendar"
        }
    }

    var nextAccessibilityLabel: String {
        switch next {
        case .timeline: "タイムラインを表示"
        case .photos: "投稿写真一覧を表示"
        case .calendar: "投稿カレンダーを表示"
        }
    }
}

@MainActor
struct RepositoryContributionCalendarView: View {
    let repository: Repository
    let activities: [RepositoryActivity]

    private let calendar: Calendar
    private let now: Date
    private let weekdayLabels = ["日", "月", "火", "水", "木", "金", "土"]

    init(
        repository: Repository,
        activities: [RepositoryActivity],
        calendar: Calendar = .current,
        now: Date = Date()
    ) {
        var calendar = calendar
        calendar.firstWeekday = 1
        self.repository = repository
        self.activities = activities
        self.calendar = calendar
        self.now = now
    }

    private var postCountsByDay: [Date: Int] {
        Dictionary(grouping: activities) { activity in
            calendar.startOfDay(for: activity.date)
        }
        .mapValues(\.count)
    }

    private var featuredPhotosByDay: [Date: ContributionPhoto] {
        let photoActivities = activities.filter {
            $0.mainPhotoURL != nil || $0.imageName != nil
        }
        let activitiesByDay = Dictionary(grouping: photoActivities) { activity in
            calendar.startOfDay(for: activity.date)
        }

        return activitiesByDay.reduce(into: [Date: ContributionPhoto]()) { result, entry in
            let sortedActivities = entry.value.sorted { lhs, rhs in
                if lhs.date != rhs.date { return lhs.date < rhs.date }
                return (lhs.backendPostID ?? 0) < (rhs.backendPostID ?? 0)
            }
            guard sortedActivities.isEmpty == false else { return }

            //  日付をseedにして、同じ日・同じ投稿集合では選択写真を安定させる。
            //  SwiftUIの再描画ごとにrandomElementすると写真がちらつくため、
            //  見た目はランダムでも再現可能なindexを使用する。
            let daySeed = UInt64(max(0, Int(entry.key.timeIntervalSince1970 / 86_400)))
            let mixedSeed = daySeed &* 6_364_136_223_846_793_005 &+ 1_442_695_040_888_963_407
            let selectedIndex = Int(mixedSeed % UInt64(sortedActivities.count))
            let selectedActivity = sortedActivities[selectedIndex]

            result[entry.key] = ContributionPhoto(
                url: selectedActivity.mainPhotoURL,
                imageName: selectedActivity.imageName
            )
        }
    }

    private var visibleMonths: [Date] {
        guard let currentMonth = calendar.date(
            from: calendar.dateComponents([.year, .month], from: now)
        ) else {
            return []
        }

        guard let firstPostDate = activities.map(\.date).min(),
              let firstPostMonth = calendar.date(
                from: calendar.dateComponents([.year, .month], from: firstPostDate)
              ) else {
            return [currentMonth]
        }

        let lastMonth = max(firstPostMonth, currentMonth)
        let monthCount = (calendar.dateComponents(
            [.month],
            from: firstPostMonth,
            to: lastMonth
        ).month ?? 0) + 1

        return (0..<monthCount).compactMap { offset in
            calendar.date(byAdding: .month, value: offset, to: firstPostMonth)
        }
    }

    var body: some View {
        ZStack {
            AppTheme.background
                .ignoresSafeArea()

            ScrollView {
                LazyVStack(alignment: .leading, spacing: 34) {
                    calendarHeader

                    ForEach(visibleMonths, id: \.self) { month in
                        ContributionMonthView(
                            month: month,
                            calendar: calendar,
                            postCountsByDay: postCountsByDay,
                            featuredPhotosByDay: featuredPhotosByDay,
                            weekdayLabels: weekdayLabels
                        )
                    }
                }
                .padding(.horizontal, 20)
                .padding(.top, 20)
                .padding(.bottom, 32)
            }
        }
    }

    private var calendarHeader: some View {
        VStack(alignment: .leading, spacing: 14) {
            VStack(alignment: .leading, spacing: 4) {
                Text("Calendar")
                    .appFont(.title)
                    .foregroundStyle(AppTheme.Text.primary)

                Text(repository.name)
                    .appFont(.sectionHeader)
                    .foregroundStyle(AppTheme.Text.low)
                    .lineLimit(1)
            }

            HStack(spacing: 8) {
                Text("Less")
                ForEach(0..<5, id: \.self) { level in
                    RoundedRectangle(cornerRadius: 3, style: .continuous)
                        .fill(ContributionColor.color(forLevel: level))
                        .frame(width: 14, height: 14)
                        .overlay {
                            RoundedRectangle(cornerRadius: 3, style: .continuous)
                                .stroke(ContributionColor.border(forLevel: level), lineWidth: 0.5)
                        }
                }
                Text("More")
            }
            .font(.system(size: 10, weight: .medium, design: .monospaced))
            .foregroundStyle(AppTheme.Text.low)
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("投稿数の色。少ないから多いまで5段階")
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

private struct ContributionMonthView: View {
    let month: Date
    let calendar: Calendar
    let postCountsByDay: [Date: Int]
    let featuredPhotosByDay: [Date: ContributionPhoto]
    let weekdayLabels: [String]

    private let gridSpacing: CGFloat = 8

    private var columns: [GridItem] {
        Array(repeating: GridItem(.flexible(), spacing: gridSpacing), count: 7)
    }

    private var monthTitle: String {
        let components = calendar.dateComponents([.year, .month], from: month)
        return "\(components.month ?? 0)月 \(components.year ?? 0)"
    }

    private var leadingEmptyCellCount: Int {
        let weekday = calendar.component(.weekday, from: month)
        return (weekday - calendar.firstWeekday + 7) % 7
    }

    private var dayCount: Int {
        calendar.range(of: .day, in: .month, for: month)?.count ?? 0
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text(monthTitle)
                .font(.system(size: 22, weight: .bold, design: .rounded))
                .foregroundStyle(AppTheme.Text.primary)

            LazyVGrid(columns: columns, spacing: gridSpacing) {
                ForEach(weekdayLabels, id: \.self) { weekday in
                    Text(weekday)
                        .font(.system(size: 11, weight: .bold, design: .rounded))
                        .foregroundStyle(AppTheme.Text.medium)
                        .frame(maxWidth: .infinity)
                }

                ForEach(0..<leadingEmptyCellCount, id: \.self) { _ in
                    Color.clear
                        .aspectRatio(1, contentMode: .fit)
                        .accessibilityHidden(true)
                }

                ForEach(1...max(dayCount, 1), id: \.self) { day in
                    if day <= dayCount, let date = date(for: day) {
                        let dayKey = calendar.startOfDay(for: date)
                        ContributionDayCell(
                            day: day,
                            date: date,
                            postCount: postCountsByDay[dayKey, default: 0],
                            featuredPhoto: featuredPhotosByDay[dayKey]
                        )
                    }
                }
            }
        }
    }

    private func date(for day: Int) -> Date? {
        calendar.date(byAdding: .day, value: day - 1, to: month)
    }
}

private struct ContributionDayCell: View {
    let day: Int
    let date: Date
    let postCount: Int
    let featuredPhoto: ContributionPhoto?

    private var level: Int {
        switch postCount {
        case 0: 0
        case 1: 1
        case 2: 2
        case 3: 3
        default: 4
        }
    }

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: 7, style: .continuous)
                .fill(ContributionColor.color(forLevel: level))

            if let featuredPhoto {
                ContributionPhotoOverlay(photo: featuredPhoto)
                    .opacity(0.24)
                    .blendMode(.luminosity)
            }

            RoundedRectangle(cornerRadius: 7, style: .continuous)
                .fill(ContributionColor.color(forLevel: level).opacity(0.16))
                .overlay {
                    RoundedRectangle(cornerRadius: 7, style: .continuous)
                        .stroke(ContributionColor.border(forLevel: level), lineWidth: 0.75)
                }

            Text("\(day)")
                .font(.system(size: 13, weight: postCount > 0 ? .bold : .medium, design: .rounded))
                .foregroundStyle(postCount > 0 ? Color.white : AppTheme.Text.regular)
        }
        .aspectRatio(1, contentMode: .fit)
        .clipShape(RoundedRectangle(cornerRadius: 7, style: .continuous))
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(accessibilityText)
    }

    private var accessibilityText: String {
        let formattedDate = date.formatted(
            .dateTime
                .locale(Locale(identifier: "ja_JP"))
                .year()
                .month()
                .day()
        )
        return postCount == 0
            ? "\(formattedDate)、投稿なし"
            : "\(formattedDate)、\(postCount)件の投稿"
    }
}

private struct ContributionPhoto: Hashable {
    let url: URL?
    let imageName: String?
}

private struct ContributionPhotoOverlay: View {
    let photo: ContributionPhoto

    var body: some View {
        GeometryReader { proxy in
            Group {
                if let url = photo.url {
                    AsyncImage(url: url) { phase in
                        if case let .success(image) = phase {
                            image
                                .resizable()
                                .scaledToFill()
                        }
                    }
                } else if let imageName = photo.imageName, UIImage(named: imageName) != nil {
                    Image(imageName)
                        .resizable()
                        .scaledToFill()
                }
            }
            .frame(width: proxy.size.width, height: proxy.size.height)
            .clipped()
        }
        .accessibilityHidden(true)
    }
}

private enum ContributionColor {
    static func color(forLevel level: Int) -> Color {
        switch level {
        case 1: Color(red: 0.055, green: 0.267, blue: 0.161) // GitHub #0e4429
        case 2: Color(red: 0.000, green: 0.427, blue: 0.196) // GitHub #006d32
        case 3: Color(red: 0.149, green: 0.651, blue: 0.255) // GitHub #26a641
        case 4: Color(red: 0.224, green: 0.827, blue: 0.325) // GitHub #39d353
        default: Color(red: 0.086, green: 0.106, blue: 0.133) // GitHub #161b22
        }
    }

    static func border(forLevel level: Int) -> Color {
        level == 0
            ? Color(red: 0.188, green: 0.216, blue: 0.255).opacity(0.55)
            : Color.white.opacity(0.08)
    }
}

#Preview {
    NavigationStack {
        RepositoryContributionCalendarView(
            repository: Repository.mockRepositories[0],
            activities: RepositoryActivity.mockActivities(for: Repository.mockRepositories[0])
        )
    }
}
