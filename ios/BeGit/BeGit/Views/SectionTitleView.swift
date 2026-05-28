//  SectionTitleView.swift
//  BeGit画面共通のセクションタイトル

import SwiftUI

//  BeGit共通Section title View
struct SectionTitleView: View {
    let title: String       //  Sectionタイトル
    let caption: String?    //  補足caption

    init(_ title: String, caption: String? = nil) {
        self.title = title
        self.caption = caption
    }

    var body: some View {
        //  Section title本体
        VStack(alignment: .leading, spacing: 4) {
            //  Sectionタイトル表示
            Text(title)
                .font(.system(size: 18, weight: .black, design: .monospaced))
                .foregroundStyle(.white)

            //  caption表示
            if let caption {
                Text(caption)
                    .font(.system(size: 12, weight: .semibold, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.48))
            }
        }
    }
}

