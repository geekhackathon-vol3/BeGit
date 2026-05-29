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
