import SwiftUI

struct CameraView: View {

    @StateObject private var camera = CameraManager()

    @State private var showPreview = false

    var body: some View {

        ZStack {

            // カメラ映像
            CameraPreview(session: camera.session)
                .ignoresSafeArea()

            // 上下グラデーション
            LinearGradient(
                colors: [
                    .black.opacity(0.5),
                    .clear,
                    .black.opacity(0.85)
                ],
                startPoint: .top,
                endPoint: .bottom
            )
            .ignoresSafeArea()

            VStack {

                // タイトル
                Text("BeGit_")
                    .font(
                        .system(
                            size: 28,
                            weight: .black,
                            design: .monospaced
                        )
                    )
                    .foregroundStyle(.white)
                    .padding(.top, 20)

                Spacer()

                // 前面カメラ風小窓
                HStack {

                    ZStack {

                        RoundedRectangle(cornerRadius: 20)
                            .fill(Color.black.opacity(0.35))

                        Image(systemName: "person.fill")
                            .font(.system(size: 30))
                            .foregroundStyle(.white.opacity(0.8))
                    }
                    .frame(width: 110, height: 150)
                    .overlay(
                        RoundedRectangle(cornerRadius: 20)
                            .stroke(Color.white.opacity(0.8), lineWidth: 2)
                    )
                    .shadow(radius: 10)

                    Spacer()
                }
                .padding(.horizontal, 20)

                Spacer()

                // シャッターボタン
                Button {

                    camera.takePhoto()

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

        // カメラ開始
        .onAppear {

            camera.startSession()
        }

        // 撮影後遷移
        .onReceive(camera.$capturedImage) { image in

            if image != nil {

                showPreview = true
            }
        }

        // プレビュー画面
        .fullScreenCover(isPresented: $showPreview) {

            if let image = camera.capturedImage {

                PhotoPreviewView(image: image)

            } else {

                ProgressView()
            }
        }
    }
}

#Preview {
    CameraView()
}
