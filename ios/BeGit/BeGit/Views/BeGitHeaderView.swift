//  BeGitHeaderView.swift
//  BeGit画面共通のロゴ付きヘッダー

import SwiftUI
import UIKit

//  BeGit共通Header View
struct BeGitHeaderView: View {
    let title: String       //  Headerタイトル
    let subtitle: String?   //  Header補足テキスト

    var body: some View {
        //  Header本体
        VStack(spacing: 4) {
            //  Headerタイトル表示
            Text(title)
                .font(.system(size: 13, weight: .bold, design: .monospaced))
                .foregroundStyle(AppTheme.softPink)
                .textCase(.uppercase)

            //  subtitle表示
            if let subtitle {
                Text(subtitle)
                    .font(.system(size: 12, weight: .semibold, design: .monospaced))
                    .foregroundStyle(.white.opacity(0.50))
                    .lineLimit(1)
                    .multilineTextAlignment(.center)
            }
        }
        .frame(maxWidth: .infinity)
    }
}

struct BeGitToolbarLogoView: View {
    var body: some View {
        Group {
            //  ロゴ画像が存在する場合
            if let image = UIImage(named: "begit_logo") {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFit()
            } else {
                //  ロゴ画像未設定時のFallback表示
                Text("BG")
                    .font(.system(size: 16, weight: .black, design: .monospaced))
                    .foregroundStyle(.black)
            }
        }
        .frame(width: 118, height: 34)
    }
}

struct BeGitBackButton: View {
    @Environment(\.dismiss) private var dismiss
    var color: Color = AppTheme.softPink
    private let titleKey = LocalizedStringKey("Back")

    var body: some View {
        Button(action: dismiss.callAsFunction) {
            HStack(spacing: 5) {
                Image("begit_back_arrow")
                    .renderingMode(.template)
                    .resizable()
                    .scaledToFit()
                    .frame(width: 22, height: 22)

                Text(titleKey)
                    .font(.system(size: 21, weight: .regular, design: .monospaced))
                    .lineLimit(1)
                    .fixedSize(horizontal: true, vertical: false)
            }
            .foregroundStyle(color)
            .frame(minWidth: 82, minHeight: 44, alignment: .leading)
        }
        .buttonStyle(.plain)
        .accessibilityLabel(Text(titleKey))
    }
}
