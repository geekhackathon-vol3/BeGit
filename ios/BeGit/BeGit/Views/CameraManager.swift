//
//  CameraManager.swift
//  BeGit
//
//  Created by 越智友香 on 2026/05/29.
//

import AVFoundation
import SwiftUI
import Combine

class CameraManager: NSObject, ObservableObject {

    // MARK: - Camera Session

    let session = AVCaptureSession()

    private let output = AVCapturePhotoOutput()

    private let sessionQueue = DispatchQueue(label: "com.begit.sessionQueue")

    // MARK: - State

    @Published var capturedImage: UIImage?
    @Published var frontCapturedImage: UIImage?

    @Published var useFrontCamera = true

    private var currentPosition: AVCaptureDevice.Position = .back
    private var pendingPhotoPositions: [Int64: AVCaptureDevice.Position] = [:]

    // MARK: - Init

    override init() {

        super.init()

        configure()
    }

    // MARK: - Configure

    private func configure() {

        session.beginConfiguration()

        // 背面カメラ
        guard let device = AVCaptureDevice.default(
            .builtInWideAngleCamera,
            for: .video,
            position: .back
        ) else {

            print("Back camera not found")

            session.commitConfiguration()
            return
        }

        do {

            let input = try AVCaptureDeviceInput(device: device)

            if session.canAddInput(input) {

                session.addInput(input)
            }

            if session.canAddOutput(output) {

                session.addOutput(output)
            }

        } catch {

            print("Camera configure error:", error)
        }

        session.commitConfiguration()
    }

    // MARK: - Session Control

    func startSession() {

        sessionQueue.async {

            guard !self.session.isRunning else {
                return
            }

            self.session.startRunning()
        }
    }

    func stopSession() {

        sessionQueue.async {

            guard self.session.isRunning else {
                return
            }

            self.session.stopRunning()
        }
    }

    // MARK: - Camera Switch

    func switchCamera(position: AVCaptureDevice.Position) {

        sessionQueue.async {

            self.session.beginConfiguration()

            // 現在のInput取得
            guard let currentInput = self.session.inputs.first as? AVCaptureDeviceInput else {

                self.session.commitConfiguration()
                return
            }

            // 新しいカメラ取得
            guard let newDevice = AVCaptureDevice.default(
                .builtInWideAngleCamera,
                for: .video,
                position: position
            ) else {

                self.session.commitConfiguration()
                return
            }

            do {

                let newInput = try AVCaptureDeviceInput(device: newDevice)

                guard self.session.canAddInput(newInput) else {

                    self.session.commitConfiguration()
                    return
                }

                // Input削除
                self.session.removeInput(currentInput)

                self.session.addInput(newInput)

            } catch {

                print("Switch camera error:", error)
                self.session.commitConfiguration()
                return
            }

            self.currentPosition = position

            self.session.commitConfiguration()
        }
    }

    // MARK: - Single Photo

    func takePhoto() {

        sessionQueue.async {

            let settings = AVCapturePhotoSettings()

            // 撮影時のカメラ位置保存
            self.pendingPhotoPositions[settings.uniqueID] = self.currentPosition

            self.output.capturePhoto(
                with: settings,
                delegate: self
            )
        }
    }

    // MARK: - BeReal Style Photo

    func takeBeRealPhoto() {

        // 前回画像をリセット
        capturedImage = nil
        frontCapturedImage = nil

        // 背面カメラへ
        switchCamera(position: .back)

        // 少し待って撮影
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {

            self.takePhoto()

            // 前面カメラも使う場合
            if self.useFrontCamera {

                // 少しズラして前面へ
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.7) {

                    self.switchCamera(position: .front)

                    // 切替待ち
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {

                        self.takePhoto()

                        // 最後に背面へ戻す
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) {

                            self.switchCamera(position: .back)
                        }
                    }
                }
            }
        }
    }
}

// MARK: - AVCapturePhotoCaptureDelegate

extension CameraManager: AVCapturePhotoCaptureDelegate {

    func photoOutput(
        _ output: AVCapturePhotoOutput,
        didFinishProcessingPhoto photo: AVCapturePhoto,
        error: Error?
    ) {

        if let error {

            print("Photo capture error:", error)
            return
        }

        guard let data = photo.fileDataRepresentation(),
              let image = UIImage(data: data)
        else {

            print("Image conversion failed")
            return
        }

        let pos = sessionQueue.sync {
            self.pendingPhotoPositions.removeValue(forKey: photo.resolvedSettings.uniqueID) ?? .back
        }

        DispatchQueue.main.async {

            // 撮影時のカメラ位置で保存先を分岐
            if pos == .back {

                self.capturedImage = image

            } else {

                self.frontCapturedImage = image
            }
        }
    }
}
