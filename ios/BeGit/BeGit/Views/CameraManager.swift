//
//  CameraManager.swift
//  BeGit
//
//  Created by 越智友香 on 2026/05/29.
//

import AVFoundation
import Combine
import SwiftUI

enum CaptureOrder: String, CaseIterable {
    case frontThenBack
    case backThenFront

    var firstPosition: AVCaptureDevice.Position {
        switch self {
        case .frontThenBack: .front
        case .backThenFront: .back
        }
    }

    var secondPosition: AVCaptureDevice.Position {
        switch self {
        case .frontThenBack: .back
        case .backThenFront: .front
        }
    }

    var displayName: String {
        switch self {
        case .frontThenBack: "内カメ → 外カメ"
        case .backThenFront: "外カメ → 内カメ"
        }
    }

    mutating func toggle() {
        self = self == .frontThenBack ? .backThenFront : .frontThenBack
    }
}

enum DualCaptureState: Equatable {
    case idle
    case preparingFirst
    case capturingFirst
    case switchingCamera
    case countdown(Int)
    case capturingSecond
    case completed
    case failed(String)
}

final class CameraManager: NSObject, ObservableObject {

    let session = AVCaptureSession()
    private let output = AVCapturePhotoOutput()

    @Published var capturedImage: UIImage?
    @Published var frontCapturedImage: UIImage?
    @Published private(set) var captureCompleted = false
    @Published private(set) var captureState: DualCaptureState = .idle
    @Published private(set) var captureOrder: CaptureOrder

    private var currentPosition: AVCaptureDevice.Position = .unspecified
    private var capturePositions: [Int64: AVCaptureDevice.Position] = [:]
    private var completedCaptureCount = 0
    private var pendingWorkItems: [DispatchWorkItem] = []

    private static let captureOrderDefaultsKey = "camera.captureOrder"
    private let cameraStabilizationDelay: TimeInterval = 0.5
    private let secondShotCountdownSeconds = 3

    var isCapturing: Bool {
        switch captureState {
        case .preparingFirst, .capturingFirst, .switchingCamera, .countdown, .capturingSecond:
            true
        case .idle, .completed, .failed:
            false
        }
    }

    override init() {
        let savedOrder = UserDefaults.standard.string(forKey: Self.captureOrderDefaultsKey)
        captureOrder = CaptureOrder(rawValue: savedOrder ?? "") ?? .frontThenBack

        super.init()

        configure(initialPosition: captureOrder.firstPosition)
    }

    private func configure(initialPosition: AVCaptureDevice.Position) {
        session.beginConfiguration()

        let preferredPosition = cameraAvailable(at: initialPosition) ? initialPosition : .back
        guard let device = cameraDevice(position: preferredPosition) else {
            print("Camera not found")
            session.commitConfiguration()
            captureState = .failed("カメラを利用できません")
            return
        }

        do {
            let input = try AVCaptureDeviceInput(device: device)

            if session.canAddInput(input) {
                session.addInput(input)
                currentPosition = preferredPosition
            }

            if session.canAddOutput(output) {
                session.addOutput(output)
            }
        } catch {
            print("Camera configure error:", error)
            captureState = .failed("カメラの準備に失敗しました")
        }

        session.commitConfiguration()
    }

    func startSession() {
        guard !session.isRunning else { return }

        DispatchQueue.global(qos: .userInitiated).async {
            self.session.startRunning()
        }
    }

    func stopSession() {
        cancelPendingWork()
        guard session.isRunning else { return }

        DispatchQueue.global(qos: .userInitiated).async {
            self.session.stopRunning()
        }
    }

    func resetForRetake() {
        cancelPendingWork()
        capturedImage = nil
        frontCapturedImage = nil
        capturePositions.removeAll()
        completedCaptureCount = 0
        captureCompleted = false
        captureState = .idle
        _ = switchCamera(position: captureOrder.firstPosition)
    }

    func toggleCaptureOrder() {
        guard !isCapturing else { return }

        var nextOrder = captureOrder
        nextOrder.toggle()

        guard cameraAvailable(at: nextOrder.firstPosition) else {
            captureState = .failed("選択したカメラを利用できません")
            return
        }

        captureOrder = nextOrder
        UserDefaults.standard.set(nextOrder.rawValue, forKey: Self.captureOrderDefaultsKey)
        captureState = .idle
        _ = switchCamera(position: nextOrder.firstPosition)
    }

    @discardableResult
    private func switchCamera(position: AVCaptureDevice.Position) -> Bool {
        if currentPosition == position {
            return true
        }

        guard let newDevice = cameraDevice(position: position),
              let newInput = try? AVCaptureDeviceInput(device: newDevice) else {
            return false
        }

        session.beginConfiguration()
        let currentInput = session.inputs.first as? AVCaptureDeviceInput

        if let currentInput {
            session.removeInput(currentInput)
        }

        guard session.canAddInput(newInput) else {
            if let currentInput, session.canAddInput(currentInput) {
                session.addInput(currentInput)
            }
            session.commitConfiguration()
            return false
        }

        session.addInput(newInput)
        currentPosition = position
        session.commitConfiguration()
        return true
    }

    func takeBeRealPhoto() {
        guard !isCapturing else { return }
        guard cameraAvailable(at: captureOrder.firstPosition),
              cameraAvailable(at: captureOrder.secondPosition) else {
            captureState = .failed("内カメと外カメの両方を利用できません")
            return
        }

        resetCaptureValues()
        captureState = .preparingFirst

        guard switchCamera(position: captureOrder.firstPosition) else {
            failCapture("1枚目のカメラへ切り替えられませんでした")
            return
        }

        schedule(after: cameraStabilizationDelay) { [weak self] in
            guard let self, self.captureState == .preparingFirst else { return }
            self.captureState = .capturingFirst
            self.takePhoto()
        }
    }

    private func takePhoto() {
        let settings = AVCapturePhotoSettings()
        capturePositions[settings.uniqueID] = currentPosition
        output.capturePhoto(with: settings, delegate: self)
    }

    private func prepareSecondCapture() {
        captureState = .switchingCamera

        guard switchCamera(position: captureOrder.secondPosition) else {
            failCapture("2枚目のカメラへ切り替えられませんでした")
            return
        }

        schedule(after: cameraStabilizationDelay) { [weak self] in
            guard let self else { return }
            self.runCountdown(self.secondShotCountdownSeconds)
        }
    }

    private func runCountdown(_ value: Int) {
        guard isCapturing else { return }

        if value == 0 {
            captureState = .capturingSecond
            takePhoto()
            return
        }

        captureState = .countdown(value)
        schedule(after: 1) { [weak self] in
            self?.runCountdown(value - 1)
        }
    }

    private func resetCaptureValues() {
        cancelPendingWork()
        capturedImage = nil
        frontCapturedImage = nil
        capturePositions.removeAll()
        completedCaptureCount = 0
        captureCompleted = false
    }

    private func failCapture(_ message: String) {
        cancelPendingWork()
        capturePositions.removeAll()
        completedCaptureCount = 0
        captureCompleted = false
        captureState = .failed("\(message) もう一度お試しください")
    }

    private func schedule(after delay: TimeInterval, action: @escaping () -> Void) {
        let workItem = DispatchWorkItem(block: action)
        pendingWorkItems.append(workItem)
        DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: workItem)
    }

    private func cancelPendingWork() {
        pendingWorkItems.forEach { $0.cancel() }
        pendingWorkItems.removeAll()
    }

    private func cameraDevice(position: AVCaptureDevice.Position) -> AVCaptureDevice? {
        AVCaptureDevice.default(
            .builtInWideAngleCamera,
            for: .video,
            position: position
        )
    }

    private func cameraAvailable(at position: AVCaptureDevice.Position) -> Bool {
        cameraDevice(position: position) != nil
    }

}

extension CameraManager: AVCapturePhotoCaptureDelegate {

    func photoOutput(
        _ output: AVCapturePhotoOutput,
        didFinishProcessingPhoto photo: AVCapturePhoto,
        error: Error?
    ) {
        let captureID = photo.resolvedSettings.uniqueID

        if let error {
            print("Photo capture error:", error)
            DispatchQueue.main.async { [weak self] in
                self?.capturePositions.removeValue(forKey: captureID)
                self?.failCapture("写真の撮影に失敗しました")
            }
            return
        }

        guard let data = photo.fileDataRepresentation(),
              let image = UIImage(data: data) else {
            DispatchQueue.main.async { [weak self] in
                self?.capturePositions.removeValue(forKey: captureID)
                self?.failCapture("写真の保存に失敗しました")
            }
            return
        }

        DispatchQueue.main.async { [weak self] in
            guard let self,
                  let position = self.capturePositions.removeValue(forKey: captureID) else {
                self?.failCapture("撮影したカメラを確認できませんでした")
                return
            }

            if position == .back {
                self.capturedImage = image
            } else {
                self.frontCapturedImage = image
            }

            self.completedCaptureCount += 1

            if self.completedCaptureCount == 1 {
                self.prepareSecondCapture()
            } else if self.completedCaptureCount == 2 {
                self.cancelPendingWork()
                self.captureState = .completed
                self.captureCompleted = true
            }
        }
    }
}
