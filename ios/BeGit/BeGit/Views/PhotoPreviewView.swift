//
//  PhotoPreviewView.swift
//  BeGit
//

import SwiftUI

struct PhotoPreviewView: View {

    @StateObject var viewModel: CreatePostViewModel
    let onPostCompleted: () -> Void

    @Environment(\.dismiss) private var dismiss

    @State private var isPosting = false

    var body: some View {

        ZStack {

            Color.black
                .ignoresSafeArea()

            VStack(spacing: 0) {

                // MARK: - Header

                HStack {

                    Button {

                        dismiss()

                    } label: {

                        Image(systemName: "xmark")
                            .font(.system(size: 18, weight: .bold))
                            .foregroundStyle(.white)
                            .frame(width: 40, height: 40)
                    }

                    Spacer()

                    Text("BeGit_")
                        .font(
                            .system(
                                size: 22,
                                weight: .black,
                                design: .monospaced
                            )
                        )
                        .foregroundStyle(.white)

                    Spacer()

                    Color.clear
                        .frame(width: 40)
                }
                .padding(.horizontal, 16)
                .padding(.top, 10)

                Spacer()

                // MARK: - Photo

                ZStack(alignment: .topLeading) {

                    // Main Photo
                    Image(uiImage: viewModel.mainImage ?? UIImage())
                        .resizable()
                        .scaledToFill()
                        .frame(maxWidth: .infinity)
                        .frame(height: 620)
                        .clipped()
                        .cornerRadius(30)

                    // Front Camera Photo
                    if let frontImage = viewModel.frontImage {

                        Image(uiImage: frontImage)
                            .resizable()
                            .scaledToFill()
                            .frame(width: 110, height: 150)
                            .clipped()
                            .cornerRadius(18)
                            .overlay(
                                RoundedRectangle(cornerRadius: 18)
                                    .stroke(
                                        Color.white.opacity(0.9),
                                        lineWidth: 2
                                    )
                            )
                            .shadow(radius: 10)
                            .padding(18)
                    }
                }
                .padding(.horizontal, 14)

                Spacer()

                // MARK: - Bottom Buttons

                HStack(spacing: 16) {

                    // Retake
                    Button {

                        dismiss()

                    } label: {

                        Text("Retake")
                            .font(.system(size: 18, weight: .bold))
                            .foregroundStyle(.white)
                            .frame(maxWidth: .infinity)
                            .frame(height: 58)
                            .background(
                                Color.white.opacity(0.12)
                            )
                            .clipShape(Capsule())
                    }

                    // Post
                    Button {
                        Task {
                            do {
                                try await viewModel.submitPost()
                                dismiss()
                                onPostCompleted()        // → NavigationStack で Result へ push
                            } catch {
                                await MainActor.run {
                                    viewModel.postError = error
                                }
                                print("Upload failed:", error)
                            }
                        }

                    } label: {

                        Text("Post")
                            .font(.system(size: 18, weight: .black))
                            .foregroundStyle(.black)
                            .frame(maxWidth: .infinity)
                            .frame(height: 58)
                            .background(.white)
                            .clipShape(Capsule())
                    }
                }
                .padding(.horizontal, 20)
                .padding(.bottom, 40)
            }
        }
    }
}

#Preview {
    PhotoPreviewView(
        viewModel: CreatePostViewModel(
            mainImage: UIImage(systemName: "photo"),
            frontImage: UIImage(systemName: "person.fill"),
            repositoryID: 1,
            repoFullName: "owner/repo",
            githubLogin: "tom",
            accessToken: ""
        ),
        onPostCompleted: {}
    )
}
