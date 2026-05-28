//  RepositoryCardView.swift
//  Repository一覧で利用するカードコンポーネント

import SwiftUI

struct RepositoryCardView: View {
    let repository: Repository           //  表示対象のRepository

    private let visibleAvatarLimit = 4  //  表示するavatar最大数

    var body: some View {
        //  Repository card本体
        HStack(spacing: 14) {
            VStack(alignment: .leading, spacing: 14) {
                //  Repository名
                Text(repository.name)
                    .font(.system(size: 18, weight: .bold, design: .monospaced))
                    .foregroundStyle(.white)
                    .lineLimit(2)
                    .minimumScaleFactor(0.82)

                HStack(spacing: 12) {
                     //  member avatar一覧
                    avatarStack

                    //  member数表示
                    Text("\(repository.memberCount) members")
                        .font(.system(size: 13, weight: .semibold, design: .monospaced))
                        .foregroundStyle(.white.opacity(0.58))
                        .lineLimit(1)
                }
            }

            Spacer(minLength: 8)

            //  詳細画面遷移アイコン
            Image(systemName: "arrow.up.right")
                .font(.system(size: 15, weight: .black))
                .foregroundStyle(AppTheme.accent)
                .frame(width: 38, height: 38)
                .background(AppTheme.accent.opacity(0.12))  //  icon背景
                .clipShape(Circle())                        //  Circle shape
        }
        .padding(18)                //  card padding
        .background(cardBackground) //  card背景
        .clipShape(RoundedRectangle(cornerRadius: 24, style: .continuous))  //  card shape
        //  card border
        .overlay(
            RoundedRectangle(cornerRadius: 24, style: .continuous)
                .stroke(Color.white.opacity(0.10), lineWidth: 1)
        )
    }

    // MARK: - Components

    //  重なり表示するavatar一覧
    private var avatarStack: some View {
        HStack(spacing: -8) {
            //  表示上限数までavatar表示
            ForEach(Array(repository.members.prefix(visibleAvatarLimit))) { member in
                AvatarView(member: member, size: 30)
                    //  avatar境界線
                    .overlay(
                        Circle()
                            .stroke(AppTheme.cardBackground, lineWidth: 2)
                    )
            }
        }
    }

    //  card背景Gradient
    private var cardBackground: some View {
        LinearGradient(
            colors: [
                AppTheme.cardBackground,
                Color(red: 0.10, green: 0.08, blue: 0.16)
            ],
            startPoint: .topLeading,
            endPoint: .bottomTrailing
        )
    }
}

//  member avatar表示View
struct AvatarView: View {
    let member: RepositoryMember    //  表示対象member 
    let size: CGFloat               //  avatarサイズ

    var body: some View {
        Group {
            //  avatar画像URLが存在する場合
            if let avatarURL = member.avatarURL {
                AsyncImage(url: avatarURL) { phase in
                    switch phase {
                    //  avatar画像読み込み成功
                    case .success(let image):
                        image
                            .resizable()
                            .scaledToFill()
                    //  読み込み中・失敗時
                    default:
                        placeholder
                    }
                }
            } else {
                //  avatar未設定時
                placeholder
            }
        }
        .frame(width: size, height: size)   //  avatar frame
        .clipShape(Circle())                //  Circle avatar
    }

    //  avatar placeholder
    private var placeholder: some View {
        Circle()
            .fill(AppTheme.accent.opacity(0.20))    //  placeholder背景
            .overlay(
                //  login頭文字表示
                Text(String(member.login.prefix(1)).uppercased())
                    .font(.system(size: size * 0.42, weight: .black, design: .monospaced))
                    .foregroundStyle(AppTheme.accent)
            )
    }
}

