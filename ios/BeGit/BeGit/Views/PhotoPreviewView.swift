//
//  PhotoPreviewView.swift
//  BeGit
//
//  Created by 越智友香 on 2026/05/29.
//

import SwiftUI

struct PhotoPreviewView: View {

    let image: UIImage

    var body: some View {

        ZStack {

            Color.black
                .ignoresSafeArea()

            VStack {

                Spacer()

                Image(uiImage: image)
                    .resizable()
                    .scaledToFit()
                    .cornerRadius(24)
                    .padding()

                Spacer()

                Button {

                    // 投稿処理など

                } label: {

                    Text("Post")
                        .font(.system(size: 20, weight: .bold))
                        .foregroundStyle(.black)
                        .frame(maxWidth: .infinity)
                        .frame(height: 60)
                        .background(.white)
                        .clipShape(Capsule())
                        .padding(.horizontal, 32)
                }
                .padding(.bottom, 40)
            }
        }
    }
}
