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

    let session = AVCaptureSession()

    private let output = AVCapturePhotoOutput()

    @Published var capturedImage: UIImage?

    override init() {

        super.init()

        configure()
    }

    private func configure() {

        session.beginConfiguration()

        guard let device = AVCaptureDevice.default(
            .builtInWideAngleCamera,
            for: .video,
            position: .back
        ) else {

            print("camera not found")
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

            print(error)
        }

        session.commitConfiguration()
    }

    func startSession() {

        guard !session.isRunning else {
            return
        }

        DispatchQueue.global(qos: .userInitiated).async {

            self.session.startRunning()
        }
    }

    func stopSession() {

        guard session.isRunning else {
            return
        }

        session.stopRunning()
    }

    func takePhoto() {

        let settings = AVCapturePhotoSettings()

        output.capturePhoto(
            with: settings,
            delegate: self
        )
    }
}

extension CameraManager: AVCapturePhotoCaptureDelegate {

    func photoOutput(
        _ output: AVCapturePhotoOutput,
        didFinishProcessingPhoto photo: AVCapturePhoto,
        error: Error?
    ) {

        guard let data = photo.fileDataRepresentation(),
              let image = UIImage(data: data)
        else {
            return
        }

        DispatchQueue.main.async {

            self.capturedImage = image
        }
    }
}
