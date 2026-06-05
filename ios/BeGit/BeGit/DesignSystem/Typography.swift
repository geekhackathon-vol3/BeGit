import SwiftUI

/// App-wide typographic tokens and a helper modifier.
enum AppTypography {
    enum TextStyle {
        case title
        case logo
        case headline
        case subheadline
        case body
        case label
        case sectionHeader
        case caption
        case small
    }

    static func font(for style: TextStyle, design: Font.Design = .monospaced) -> Font {
        switch style {
        case .title:
            return .custom("Bitcount", size: 34)
        case .logo:
            return .system(size: 18, weight: .black, design: design)
        case .headline:
            return .system(size: 18, weight: .semibold, design: design)
        case .subheadline:
            return .system(size: 15, weight: .medium, design: design)
        case .body:
            return .system(size: 14, weight: .regular, design: design)
        case .label:
            return .system(size: 13, weight: .bold, design: design)
        case .sectionHeader:
            return .system(size: 12, weight: .bold, design: design)
        case .caption:
            return .system(size: 11, weight: .semibold, design: design)
        case .small:
            return .system(size: 9, weight: .regular, design: design)
        }
    }

    struct FontModifier: ViewModifier {
        let style: TextStyle
        let design: Font.Design

        func body(content: Content) -> some View {
            content.font(AppTypography.font(for: style, design: design))
        }
    }
}

extension View {
    func appFont(_ style: AppTypography.TextStyle, design: Font.Design = .monospaced) -> some View {
        modifier(AppTypography.FontModifier(style: style, design: design))
    }
}
