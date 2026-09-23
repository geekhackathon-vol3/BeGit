//
//  CameraView.swift
//  BeGit
//

import SwiftUI

struct CameraView: View {

    let repositoryID: Int64
    let repoFullName: String
    let githubLogin: String
    let accessToken: String
    var notificationID: Int64? = nil
    var initialPostType: RepositoryActivityType = .commit
    //  ② Nice Work! の下書き投稿ID。指定時は撮影した写真をこの下書きに付けて確定する
    var draftPostID: Int64? = nil
    let onPostCompleted: (RepositoryActivity?) -> Void

    @StateObject private var camera = CameraManager()
    @State private var showPreview = false

    var body: some View {

        ZStack {

            // MARK: - Camera Preview

            CameraPreview(session: camera.session)
                .ignoresSafeArea()

            // MARK: - Gradient Overlay

            LinearGradient(
                colors: [
                    .black.opacity(0.45),
                    .clear,
                    .black.opacity(0.9)
                ],
                startPoint: .top,
                endPoint: .bottom
            )
            .ignoresSafeArea()

            // MARK: - UI

            VStack {

                // Header

                ZStack {
                    Text("BeGit;")
                        .font(
                            .system(
                                size: 28,
                                weight: .black,
                                design: .monospaced
                            )
                        )
                        .foregroundStyle(AppTheme.Text.primary)
                        .frame(maxWidth: .infinity)

                    HStack {
                        BeGitBackButton(color: .white)
                        Spacer()
                    }
                }
                .padding(.horizontal, 20)
                .padding(.top, 14)

                Spacer()

                frontCameraToggle
                .padding(.horizontal, 24)
                .padding(.bottom, 24)

                // Shutter Button
                Button {
                    camera.takeBeRealPhoto()
                } label: {
                    ZStack {
                        Circle()
                            .fill(AppTheme.Text.primary)
                            .frame(width: 86, height: 86)

                        Circle()
                            .stroke(.black, lineWidth: 4)
                            .frame(width: 68, height: 68)
                    }
                }
                .disabled(camera.isCapturing)
                .opacity(camera.isCapturing ? 0.55 : 1)
                .accessibilityLabel(camera.isCapturing ? "撮影中" : "写真を撮影")
                .padding(.bottom, 34)
            }
        }
        .navigationBarBackButtonHidden()

        // MARK: - Start Camera

        .onAppear {
            camera.startSession()
        }
        .onDisappear {
            camera.stopSession()
        }

        .onChange(of: camera.captureState) { _, state in
            playFeedback(for: state)
        }

        // MARK: - Show Preview

        .onReceive(camera.$captureCompleted) { completed in
            guard completed else { return }
            showPreview = true
        }

        // MARK: - Preview Screen

        .fullScreenCover(isPresented: $showPreview, onDismiss: {
            camera.resetForRetake()
            camera.startSession()
        }) {
            if let mainImage = camera.capturedImage {
                let vm = CreatePostViewModel(
                    mainImage: mainImage,
                    frontImage: camera.frontCapturedImage,
                    repositoryID: repositoryID,
                    repoFullName: repoFullName,
                    githubLogin: githubLogin,
                    accessToken: accessToken,
                    notificationID: notificationID,
                    initialPostType: initialPostType,
                    draftPostID: draftPostID
                )

                PhotoPreviewView(
                    viewModel: vm,
                    onPostCompleted: onPostCompleted
                )
            } else {
                ProgressView()
            }
        }
    }

    private var frontCameraToggle: some View {
        HStack {
            Toggle(isOn: Binding(
                get: { camera.captureOrder == .frontThenBack },
                set: { shouldUseFrontCameraFirst in
                    let isFrontCameraFirst = camera.captureOrder == .frontThenBack
                    if shouldUseFrontCameraFirst != isFrontCameraFirst {
                        camera.toggleCaptureOrder()
                    }
                }
            )) {
                Label("Front Camera", systemImage: "camera.rotate")
                    .foregroundStyle(AppTheme.Text.primary)
                    .appFont(.subheadline)
            }
            .tint(AppTheme.accent)
            .disabled(camera.isCapturing)
            .accessibilityHint("オンでは内カメ、オフでは外カメから撮影します")
        }
    }

    private func playFeedback(for state: DualCaptureState) {
        switch state {
        case .capturingFirst, .capturingSecond:
            UIImpactFeedbackGenerator(style: .medium).impactOccurred()
        case .countdown:
            UISelectionFeedbackGenerator().selectionChanged()
        case .completed:
            UINotificationFeedbackGenerator().notificationOccurred(.success)
        case .failed:
            UINotificationFeedbackGenerator().notificationOccurred(.error)
        default:
            break
        }
    }

}

#Preview {
    CameraView(
        repositoryID: 1,
        repoFullName: "owner/repo",
        githubLogin: "tom",
        accessToken: "",
        onPostCompleted: { _ in } 
    )
}
