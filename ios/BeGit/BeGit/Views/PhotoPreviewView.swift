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
    @FocusState private var isCommentFocused: Bool

    var body: some View {

        ZStack {

            Color.black
                .ignoresSafeArea()
                .onTapGesture { isCommentFocused = false }

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

                // MARK: - Comment

                HStack {

                    Image(systemName: "text.bubble")
                        .font(.system(size: 16, weight: .semibold))
                        .foregroundStyle(.white.opacity(0.7))

                    TextField(
                        "",
                        text: $viewModel.bodyText,
                        prompt: Text("Add comment...")
                            .foregroundColor(.white.opacity(0.5))
                    )
                    .font(.system(size: 16))
                    .foregroundStyle(.white)
                    .focused($isCommentFocused)
                    .submitLabel(.done)
                    .onSubmit { isCommentFocused = false }

                    Spacer(minLength: 0)
                }
                .padding(.horizontal, 18)
                .padding(.vertical, 12)
                .background(Color.white.opacity(0.12))
                .clipShape(Capsule())
                .padding(.horizontal, 20)
                .padding(.bottom, 16)

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
