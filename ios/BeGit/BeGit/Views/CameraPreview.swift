//
//  CameraPreview.swift
//  BeGit
//
//  Created by 越智友香 on 2026/05/29.
//
import SwiftUI
import AVFoundation

struct CameraPreview: UIViewRepresentable {

    let session: AVCaptureSession

    func makeUIView(context: Context) -> PreviewView {

        let view = PreviewView()

        view.previewLayer.session = session
        view.previewLayer.videoGravity = .resizeAspectFill

        return view
    }

    func updateUIView(
        _ uiView: PreviewView,
        context: Context
    ) {

        DispatchQueue.main.async {

            uiView.previewLayer.frame = uiView.bounds
        }
    }
}

final class PreviewView: UIView {

    override class var layerClass: AnyClass {

        AVCaptureVideoPreviewLayer.self
    }

    var previewLayer: AVCaptureVideoPreviewLayer {

        layer as! AVCaptureVideoPreviewLayer
    }

    override func layoutSubviews() {

        super.layoutSubviews()

        previewLayer.frame = bounds
    }
}
