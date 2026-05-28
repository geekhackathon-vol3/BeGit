//  PrimaryButton.swift
//  BeGit共通のCapsule型プライマリボタン

import SwiftUI

struct PrimaryButton: View {
    let title: String           //  ボタンタイトル
    let systemImage: String?    //  SF Symbols icon名
    let isEnabled: Bool         //  ボタン有効状態
    let action: () -> Void      //  ボタン押下時の処理

    init(
        _ title: String,
        systemImage: String? = nil,
        isEnabled: Bool = true,
        action: @escaping () -> Void
    ) {
        self.title = title
        self.systemImage = systemImage
        self.isEnabled = isEnabled
        self.action = action
    }

    var body: some View {
        //  Primary button本体
        Button(action: action) {
            PrimaryCapsuleButtonLabel(
                title: title,
                systemImage: systemImage,
                isEnabled: isEnabled
            )
        }
        .buttonStyle(.plain)            //  デフォルトbutton style無効化
        .disabled(isEnabled == false)   //  button interaction制御
        .opacity(isEnabled ? 1 : 0.6)   //  disabled時の透明度調整
    }
}

struct PrimaryCapsuleButtonLabel: View {
    let title: String           //  ボタンタイトル
    let systemImage: String?    //  SF Symbols icon名
    let isEnabled: Bool         //  ボタン有効状態

    var body: some View {
        //  button label本体
        HStack(spacing: 10) {
            //  SF Symbols icon表示
            if let systemImage {
                Image(systemName: systemImage)
                    .font(.system(size: 16, weight: .bold))
            }

            //  ボタンタイトル
            Text(title)
                .font(.system(size: 16, weight: .bold, design: .monospaced))
        }
        .foregroundStyle(.black)
        .frame(maxWidth: .infinity) //  横幅最大
        .frame(height: 56)          //  ボタン高さ
        .background(AppTheme.accent.opacity(isEnabled ? 1 : 0.45))  //  disabled時は透明度を下げる
        .clipShape(Capsule())
    }
}
