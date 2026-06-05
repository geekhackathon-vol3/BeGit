//
//  CameraView.swift
//  BeGit
//

import SwiftUI

struct CameraView: View {

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
                HStack {
                    BeGitBackButton()

                    Spacer()

                    Text("BeGit_")
                        .font(
                            .system(
                                size: 28,
                                weight: .black,
                                design: .monospaced
                            )
                        )
                        .foregroundStyle(.white)

                    Spacer()
                        .frame(minWidth: 82)
                }
                .padding(.horizontal, 20)
                .padding(.top, 14)

                Spacer()

                // Front camera ON/OFF
                HStack {

                    Toggle(isOn: $camera.useFrontCamera) {

                        Label(
                            "Front Camera",
                            systemImage: "camera.rotate"
                        )
                        .foregroundStyle(.white)
                        .font(.system(size: 16, weight: .bold))
                    }
                    .tint(.white)
                }
                .padding(.horizontal, 24)
                .padding(.bottom, 24)

                // Shutter Button
                Button {

                    camera.takeBeRealPhoto()

                } label: {

                    ZStack {

                        Circle()
                            .fill(.white)
                            .frame(width: 86, height: 86)

                        Circle()
                            .stroke(.black, lineWidth: 4)
                            .frame(width: 68, height: 68)
                    }
                }
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

        // MARK: - Show Preview

        .onReceive(camera.$capturedImage) { image in

            if image != nil {

                DispatchQueue.main.asyncAfter(deadline: .now() + 1.2) {

                    showPreview = true
                }
            }
        }

        // MARK: - Preview Screen

        .fullScreenCover(isPresented: $showPreview) {

            if let mainImage = camera.capturedImage {

                PhotoPreviewView(
                    mainImage: mainImage,
                    frontImage: camera.frontCapturedImage
                )

            } else {

                ProgressView()
            }
        }
    }
}

#Preview {
    CameraView()
}
