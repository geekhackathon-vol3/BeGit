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
        HStack(spacing: 14) {
            //  BeGitロゴ表示
            logoView

            VStack(alignment: .leading, spacing: 4) {
                //  アプリロゴテキスト
                Text("BeGit_")
                    .font(.system(size: 28, weight: .black, design: .monospaced))
                    .foregroundStyle(.white)

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
                }
            }

            Spacer(minLength: 0)
        }
    }

    //  BeGitロゴView
    private var logoView: some View {
        Group {
            //  ロゴ画像が存在する場合
            if let image = UIImage(named: "begit_logo") {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFit()
                    .padding(7)
            } else {
                //  ロゴ画像未設定時のFallback表示
                Text("BG")
                    .font(.system(size: 16, weight: .black, design: .monospaced))
                    .foregroundStyle(.black)
            }
        }
        .frame(width: 54, height: 54)                                           //  ロゴサイズ
        .background(AppTheme.accent)                                            //  ロゴ背景色
        .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))      //  ロゴShape
    }
}

