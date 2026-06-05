import SwiftUI

// Centralized design tokens for colors used across the app.
enum AppTheme {
    // App background colors
    static let background = Color(red: 0, green: 0, blue: 0)
    static let cardBackground = Color(red: 0.247, green: 0.247, blue: 0.286)
    static let repositoryCardBackground = Color(red: 0.267, green: 0.267, blue: 0.267)
    static let fieldBackground = Color.white.opacity(0.07)
    
    static let borderSubtle = Color(red: 0.310, green: 0.322, blue: 0.357)

    // Accent colors
    static let accent = Color(red: 0.804, green: 0.718, blue: 0.965)
    static let softPink = Color(red: 1.00, green: 0.72, blue: 0.84)
    static let checkmarkGreen = Color(red: 0.725, green: 0.976, blue: 0.902)

    // Section title colors
    static let sectionYellow = Color(red: 0.980, green: 0.973, blue: 0.780)
    static let sectionPink = Color(red: 0.929, green: 0.784, blue: 0.827)
    
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
