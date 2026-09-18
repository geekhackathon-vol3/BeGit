//
//  PhotoPreviewView.swift
//  BeGit
//

import SwiftUI

struct PhotoPreviewView: View {

    @StateObject var viewModel: CreatePostViewModel
    let onPostCompleted: (RepositoryActivity?) -> Void

    @Environment(\.dismiss) private var dismiss

    @State private var isPosting = false
    @State private var isPhotoSwapped = false
    @State private var isGitHubActivityPickerPresented = false
    @FocusState private var isCommentFocused: Bool

    var body: some View {

        ZStack {

            AppTheme.background
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
                            .foregroundStyle(AppTheme.Text.primary)
                            .frame(width: 40, height: 40)
                    }

                    Spacer()

                    Text("BeGit;")
                        .appFont(.logo)
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
                    Image(uiImage: displayedMainImage)
                        .resizable()
                        .scaledToFill()
                        .frame(maxWidth: .infinity)
                        .frame(height: 620)
                        .clipped()
                        .cornerRadius(12)

                    // Front Camera Photo
                    if let thumbnailImage = displayedThumbnailImage {

                        Image(uiImage: thumbnailImage)
                            .resizable()
                            .scaledToFill()
                            .frame(width: 72, height: 96)
                            .clipped()
                            .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
                            .overlay(
                                RoundedRectangle(cornerRadius: 6, style: .continuous)
                                    .stroke(Color.black, lineWidth: 2)
                            )
                            .shadow(color: .black.opacity(0.32), radius: 10, x: 0, y: 5)
                            .padding(16)
                            .contentShape(Rectangle())
                            .onTapGesture {
                                withAnimation(.easeInOut(duration: 0.22)) {
                                    isPhotoSwapped.toggle()
                                }
                            }
                            .accessibilityLabel(isPhotoSwapped ? "外カメラを大きく表示" : "内カメラを大きく表示")
                            .accessibilityAddTraits(.isButton)
                    }
                }
                .overlay(alignment: .bottom) {
                    VStack(spacing: 8) {
                        postTypePicker
                        if viewModel.isPostTypeSelectionEnabled,
                           viewModel.selectedType != .memo,
                           viewModel.contentSource == .github {
                            githubAttachmentButton
                        }
                    }
                        .padding(.horizontal, 16)
                        .padding(.bottom, 6)
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
                        prompt: Text(commentPrompt)
                            .foregroundColor(.white.opacity(0.5))
                    )
                    .font(.system(size: 16))
                    .foregroundStyle(.white)
                    .focused($isCommentFocused)
                    .submitLabel(.done)
                    .onSubmit { isCommentFocused = false }

                    Spacer(minLength: 0)

                    if viewModel.isPostTypeSelectionEnabled,
                       viewModel.selectedType != .memo,
                       viewModel.contentSource == .manual {
                        Button {
                            presentGitHubPicker()
                        } label: {
                            Image(systemName: "point.3.connected.trianglepath.dotted")
                                .font(.system(size: 15, weight: .bold))
                                .foregroundStyle(viewModel.selectedType.pickerTint)
                                .frame(width: 32, height: 32)
                                .background(Color.white.opacity(0.08))
                                .clipShape(Circle())
                        }
                        .buttonStyle(.plain)
                        .accessibilityLabel("GitHubデータを選ぶ")
                    }
                }
                .padding(.horizontal, 18)
                .padding(.vertical, 12)
                .background(Color.white.opacity(0.12))
                .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                .padding(.horizontal, 20)
                .padding(.bottom, 16)

                // MARK: - Bottom Buttons

                HStack(spacing: 16) {

                    // Retake
                    Button {

                        dismiss()

                    } label: {

                        Text("Retake")
                            .appFont(.headline)
                            .foregroundStyle(.white)
                            .frame(maxWidth: .infinity)
                            .frame(height: 58)
                            .background(
                                AppTheme.fieldBackground
                            )
                            .clipShape(Capsule())
                    }

                    // Post
                    Button {
                        Task {
                            do {
                                try await viewModel.submitPost()
                                dismiss()
                                onPostCompleted(viewModel.postedActivity)  // → NavigationStack で Result へ push
                            } catch {
                                await MainActor.run {
                                    viewModel.postError = error
                                }
                                print("Upload failed:", error)
                            }
                        }

                    } label: {

                        Text("Post")
                            .appFont(.headline)
                            .foregroundStyle(.black)
                            .frame(maxWidth: .infinity)
                            .frame(height: 58)
                            .background(AppTheme.Text.primary)
                            .clipShape(Capsule())
                    }
                    .disabled(viewModel.canSubmit == false)
                    .opacity(viewModel.canSubmit ? 1 : 0.45)
                }
                .padding(.horizontal, 20)
                .padding(.bottom, 40)
            }
        }
        .sheet(isPresented: $isGitHubActivityPickerPresented) {
            GitHubActivityPickerView(viewModel: viewModel)
        }
    }

    private var displayedMainImage: UIImage {
        if isPhotoSwapped, let frontImage = viewModel.frontImage {
            return frontImage
        }
        return viewModel.mainImage ?? UIImage()
    }

    private var displayedThumbnailImage: UIImage? {
        guard let frontImage = viewModel.frontImage else { return nil }
        if isPhotoSwapped {
            return viewModel.mainImage
        }
        return frontImage
    }

    private var postTypePicker: some View {
        HStack(spacing: 8) {
            ForEach(RepositoryActivityType.allCases, id: \.self) { type in
                let isSelected = viewModel.selectedType == type

                Button {
                    viewModel.selectPostType(type)
                } label: {
                    HStack(spacing: 5) {
                        Image(systemName: type.pickerSystemImage)
                            .font(.system(size: 12, weight: .bold))
                        Text(type.displayName)
                            .font(.system(size: 13, weight: .bold, design: .monospaced))
                        if isSelected && viewModel.isPostTypeSelectionEnabled == false {
                            Image(systemName: "lock.fill")
                                .font(.system(size: 9, weight: .bold))
                        }
                    }
                    .foregroundStyle(isSelected ? Color.black : Color.white)
                    .frame(maxWidth: .infinity)
                    .frame(height: 38)
                    .background(isSelected ? type.pickerTint : Color.black.opacity(0.55))
                    .clipShape(Capsule())
                    .overlay {
                        Capsule()
                            .stroke(Color.white.opacity(isSelected ? 0 : 0.24), lineWidth: 1)
                    }
                }
                .buttonStyle(.plain)
                .disabled(viewModel.isPostTypeSelectionEnabled == false)
                .accessibilityLabel("投稿タイプ: \(type.displayName)")
                .accessibilityValue(isSelected ? "選択中" : "未選択")
            }
        }
    }

    private var githubAttachmentButton: some View {
        HStack(spacing: 0) {
            Button {
                presentGitHubPicker()
            } label: {
                HStack(spacing: 8) {
                    Image(systemName: viewModel.selectedGitHubActivityTitle == nil
                          ? "point.3.connected.trianglepath.dotted"
                          : "checkmark.circle.fill")
                    Text(viewModel.selectedGitHubActivityTitle ?? githubSelectionPrompt)
                        .lineLimit(1)
                    Spacer(minLength: 4)
                    Image(systemName: "chevron.right")
                        .font(.system(size: 11, weight: .bold))
                }
                .font(.system(size: 12, weight: .semibold))
                .foregroundStyle(.black)
                .padding(.leading, 13)
                .padding(.trailing, viewModel.selectedGitHubActivityTitle == nil ? 13 : 6)
                .frame(maxWidth: .infinity, minHeight: 38)
            }
            .buttonStyle(.plain)

            if viewModel.selectedGitHubActivityTitle != nil {
                Button {
                    viewModel.clearSelectedGitHubActivity()
                } label: {
                    Image(systemName: "xmark")
                        .font(.system(size: 11, weight: .bold))
                        .foregroundStyle(.black.opacity(0.62))
                        .frame(width: 38, height: 38)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("選択したGitHubデータを外す")
            }
        }
        .background(Color.white.opacity(0.92))
        .clipShape(Capsule())
    }

    private func presentGitHubPicker() {
        isCommentFocused = false
        viewModel.selectContentSource(.github)
        isGitHubActivityPickerPresented = true
    }

    private var githubSelectionPrompt: String {
        viewModel.selectedType == .commit ? "最近のcommitを選ぶ" : "最近のPRを選ぶ"
    }

    private var commentPrompt: String {
        if viewModel.selectedType == .memo || viewModel.contentSource == .manual {
            return "Add comment (required)..."
        }
        return "Add comment..."
    }
}

extension RepositoryActivityType {
    var pickerTint: Color {
        switch self {
        case .commit: Color(red: 0.45, green: 0.94, blue: 0.67)
        case .pullRequest: Color(red: 1.00, green: 0.47, blue: 0.65)
        case .memo: Color(red: 0.47, green: 0.74, blue: 1.00)
        }
    }

    var pickerSystemImage: String {
        switch self {
        case .commit: "checkmark.seal"
        case .pullRequest: "arrow.triangle.pull"
        case .memo: "hand.raised"
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
        onPostCompleted: { _ in }
    )
}
