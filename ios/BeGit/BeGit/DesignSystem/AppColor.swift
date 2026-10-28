import SwiftUI

// Centralized design tokens for colors used across the app.
enum AppTheme {
    static let storageKey = "selectedAppTheme"

    enum Theme: String, CaseIterable, Identifiable {
        case violet
        case midnight

        var id: String { rawValue }

        var name: String {
            switch self {
            case .violet:
                return "Violet"
            case .midnight:
                return "Midnight"
            }
        }

        var description: String {
            switch self {
            case .violet:
                return "Purple & Pink"
            case .midnight:
                return "Blue & Cyan"
            }
        }

        var palette: Palette {
            switch self {
            case .violet:
                return .violet
            case .midnight:
                return .midnight
            }
        }
    }

    struct Palette {
        let background: Color
        let cardBackground: Color
        let repositoryCardBackground: Color
        let fieldBackground: Color
        let borderSubtle: Color
        let accent: Color
        let softPink: Color
        let checkmarkGreen: Color
        let sectionYellow: Color
        let sectionPink: Color

        // Existing Violet theme.
        static let violet = Palette(
            background: Color(red: 0, green: 0, blue: 0),
            cardBackground: Color(red: 0.247, green: 0.247, blue: 0.286),
            repositoryCardBackground: Color(red: 0.267, green: 0.267, blue: 0.267),
            fieldBackground: Color.white.opacity(0.07),
            borderSubtle: Color(red: 0.310, green: 0.322, blue: 0.357),
            accent: Color(red: 0.804, green: 0.718, blue: 0.965),
            softPink: Color(red: 1.00, green: 0.72, blue: 0.84),
            checkmarkGreen: Color(red: 0.725, green: 0.976, blue: 0.902),
            sectionYellow: Color(red: 0.980, green: 0.973, blue: 0.780),
            sectionPink: Color(red: 0.929, green: 0.784, blue: 0.827)
        )

        // Dark blue base with blue and cyan accents.
        static let midnight = Palette(
            background: Color(red: 0.027, green: 0.043, blue: 0.071),
            cardBackground: Color(red: 0.110, green: 0.145, blue: 0.200),
            repositoryCardBackground: Color(red: 0.133, green: 0.176, blue: 0.239),
            fieldBackground: Color(red: 0.408, green: 0.722, blue: 1.000).opacity(0.09),
            borderSubtle: Color(red: 0.212, green: 0.275, blue: 0.357),
            accent: Color(red: 0.408, green: 0.722, blue: 1.000),
            softPink: Color(red: 0.478, green: 0.843, blue: 1.000),
            checkmarkGreen: Color(red: 0.400, green: 0.839, blue: 0.757),
            sectionYellow: Color(red: 1.000, green: 0.820, blue: 0.400),
            sectionPink: Color(red: 0.557, green: 0.659, blue: 1.000)
        )
    }

    static var selectedTheme: Theme {
        let rawValue = UserDefaults.standard.string(forKey: storageKey)
        return rawValue.flatMap(Theme.init(rawValue:)) ?? .violet
    }

    static var current: Palette {
        selectedTheme.palette
    }

    // App background colors
    static var background: Color { current.background }
    static var cardBackground: Color { current.cardBackground }
    static var repositoryCardBackground: Color { current.repositoryCardBackground }
    static var fieldBackground: Color { current.fieldBackground }
    static var borderSubtle: Color { current.borderSubtle }

    // Accent colors
    static var accent: Color { current.accent }
    static var softPink: Color { current.softPink }
    static var checkmarkGreen: Color { current.checkmarkGreen }

    // Section title colors
    static var sectionYellow: Color { current.sectionYellow }
    static var sectionPink: Color { current.sectionPink }

    // Text color tokens (use these instead of raw opacity values)
    enum Text {
        static let primary = Color.white
        static let high = Color.white.opacity(0.72)
        static let medium = Color.white.opacity(0.64)
        static let regular = Color.white.opacity(0.58)
        static let low = Color.white.opacity(0.50)
        static let muted = Color.white.opacity(0.30)
        static let disabled = Color.white.opacity(0.42)
    }

    // Helper for subtle backgrounds used under avatars, cards, etc.
    static func backgroundOpacity(_ value: Double) -> Color {
        background.opacity(value)
    }
}
