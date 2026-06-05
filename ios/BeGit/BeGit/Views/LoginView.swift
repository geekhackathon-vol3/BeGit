//  LoginView.swift
//  GitHubログイン画面のSwiftUI View

import SwiftUI
import UIKit

@MainActor
struct LoginView: View {
    @StateObject private var viewModel: LoginViewModel

    init() {
        _viewModel = StateObject(wrappedValue: LoginViewModel.makeDefault())
    }

    init(viewModel: LoginViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    var body: some View {
        NavigationStack {
            GeometryReader { proxy in
                ZStack {
                    AppTheme.background
                        .ignoresSafeArea()

                    VStack(spacing: 24) {
                        Spacer(minLength: proxy.size.height * 0.08)

                        logoSection

                        signInButton
                            .padding(.top, 20)

                        Spacer(minLength: 12)

                        Spacer(minLength: proxy.size.height * 0.12)
                    }
                    .frame(maxWidth: min(proxy.size.width - 32, 460))
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .padding(.horizontal, 16)
                    .padding(.vertical, 24)
                    .safeAreaPadding(.vertical, 8)
                }
            }
            .alert(item: $viewModel.alertContext) { context in
                Alert(
                    title: Text(context.title),
                    message: Text(context.message),
                    dismissButton: .default(Text("OK"), action: viewModel.dismissAlert)
                )
            }
        }
    }

    private var logoSection: some View {
        VStack(spacing: 2) {
            if let image = UIImage(named: "begit_logo") {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFit()
                    .frame(maxWidth: 320, maxHeight: 150)
            } else {
                RoundedRectangle(cornerRadius: 28, style: .continuous)
                    .fill(AppTheme.accent)
                    .frame(width: 124, height: 124)
                    .overlay(
                        Text("BeGit")
                            .appFont(.logo, design: .rounded)
                            .foregroundStyle(.black)
                    )
            }

            Text("Real-time development or nothing.")
                .appFont(.headline, design: .rounded)
                .multilineTextAlignment(.center)
                .foregroundStyle(AppTheme.Text.primary)
                .lineLimit(1)
                .minimumScaleFactor(0.72)
        }
    }

    private var signInButton: some View {
        Button(action: viewModel.signInWithGitHub) {
            HStack(spacing: 14) {
                Image("github_sign_in_icon")
                    .resizable()
                    .scaledToFit()
                    .frame(width: 24, height: 24)

                Text("[Sign in with GitHub]")
                    .appFont(.headline, design: .rounded)
                    .foregroundStyle(.black)
            }
            .frame(maxWidth: .infinity)
            .frame(height: 64)
            .background(AppTheme.accent)
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
