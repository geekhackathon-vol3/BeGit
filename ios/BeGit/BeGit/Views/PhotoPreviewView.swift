//
//  PhotoPreviewView.swift
//  BeGit
//

import SwiftUI

struct PhotoPreviewView: View {

    let mainImage: UIImage
    let frontImage: UIImage?

    @Environment(\.dismiss) private var dismiss

    var body: some View {

        ZStack {

            AppTheme.background
                .ignoresSafeArea()

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
                    Image(uiImage: mainImage)
                        .resizable()
                        .scaledToFill()
                        .frame(maxWidth: .infinity)
                        .frame(height: 620)
                        .clipped()
                        .cornerRadius(30)

                    // Front Camera Photo
                    if let frontImage {

                        Image(uiImage: frontImage)
                            .resizable()
                            .scaledToFill()
                            .frame(width: 110, height: 150)
                            .clipped()
                            .cornerRadius(18)
                            .overlay(
                                RoundedRectangle(cornerRadius: 18)
                                    .stroke(
                                        AppTheme.Text.primary.opacity(0.9),
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

                        // TODO:
                        // 投稿処理

                    } label: {

                        Text("Post")
                            .appFont(.headline)
                            .foregroundStyle(.black)
                            .frame(maxWidth: .infinity)
                            .frame(height: 58)
                            .background(AppTheme.Text.primary)
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
        mainImage: UIImage(systemName: "photo")!,
        frontImage: UIImage(systemName: "person.fill")!
    )
}
