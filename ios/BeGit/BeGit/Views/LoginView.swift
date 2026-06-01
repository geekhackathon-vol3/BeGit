//  LoginView.swift
//  GitHubログイン画面のSwiftUI View

import SwiftUI
import UIKit

//  GitHubログイン画面
@MainActor
struct LoginView: View {
    @StateObject private var viewModel: LoginViewModel  //  ログイン画面の状態と処理を管理するViewModel

    //  通常利用時はデフォルトのViewModelを生成
    init() {
        _viewModel = StateObject(wrappedValue: LoginViewModel.makeDefault())
    }

    //  テスト・Preview用にViewModelを外部注入
    init(viewModel: LoginViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        NavigationStack {
            GeometryReader { proxy in
                ZStack {
                    Color.black
                        .ignoresSafeArea()

                    VStack(spacing: 24) {
                        Spacer(minLength: proxy.size.height * 0.08)

                        logoSection

                        signInButton
                            .padding(.top, 20)

                        debugCameraButton

                        EmptyView()
                            .padding(.top, 12)

                        Spacer(minLength: proxy.size.height * 0.12)
                    }
                    .frame(maxWidth: min(proxy.size.width - 32, 460))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .padding(.horizontal, 16)
                    .padding(.vertical, 24)
                    .safeAreaPadding(.vertical, 8)
                }
            }
            // alertはそのまま
            .alert(item: $viewModel.alertContext) { context in
                Alert(
                    title: Text(context.title),
                    message: Text(context.message),
                    dismissButton: .default(Text("OK"), action: viewModel.dismissAlert)
                )
            }
        }
    }

    //  ロゴとキャッチコピー
    private var logoSection: some View {
        VStack(spacing: 2) {
            if let image = UIImage(named: "begit_logo") {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFit()
                    .frame(maxWidth: 240, maxHeight: 84)
            } else {
                RoundedRectangle(cornerRadius: 28, style: .continuous)
                    .fill(Color(red: 0.80, green: 0.72, blue: 0.96))
                    .frame(width: 124, height: 124)
                    .overlay(
                        Text("BeGit")
                            .font(.system(size: 28, weight: .black, design: .rounded))
                            .foregroundStyle(.black)
                    )
            }

            Text("Real-time development or nothing.")
                .font(.system(size: 18, weight: .regular, design: .rounded))
                .multilineTextAlignment(.center)
                .foregroundStyle(.white)
                .lineLimit(1)
                .minimumScaleFactor(0.72)
        }
    }

    //  GitHubログイン開始ボタン
    private var signInButton: some View {
        Button(action: viewModel.signInWithGitHub) {
            HStack(spacing: 14) {
                Image("github_sign_in_icon")
                    .resizable()
                    .scaledToFit()
                    .frame(width: 24, height: 24)

                Text("[Sign in with GitHub]")
                    .font(.system(size: 20, weight: .bold, design: .rounded))
                    .foregroundStyle(.black)
            }
            .frame(maxWidth: .infinity)
            .frame(height: 64)
            .background(Color(red: 0.804, green: 0.718, blue: 0.965))
            .clipShape(Capsule())
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("github_sign_in_button")
    }

}

struct LoginView_iPhoneSE_Previews: PreviewProvider {
    static var previews: some View {
        LoginView()
            .previewDevice("iPhone SE (3rd generation)")
    }
}

struct LoginView_iPhone16ProMax_Previews: PreviewProvider {
    static var previews: some View {
        LoginView()
            .previewDevice("iPhone 16 Pro Max")
    }
}

// デバッグ用カメラ遷移ボタン
private var debugCameraButton: some View {

    NavigationLink {

        CameraView()

    } label: {

        Text("Debug Camera")
            .font(.system(size: 16, weight: .semibold))
            .foregroundStyle(.white.opacity(0.8))
            .padding(.horizontal, 20)
            .padding(.vertical, 12)
            .background(
                RoundedRectangle(cornerRadius: 999)
                    .fill(Color.white.opacity(0.08))
            )
            .overlay(
                RoundedRectangle(cornerRadius: 999)
                    .stroke(Color.white.opacity(0.12), lineWidth: 1)
            )
    }
}
