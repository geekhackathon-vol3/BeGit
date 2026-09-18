//  ActiveChallengeBannerView.swift
//  進行中の BeGit Time! を Dashboard 上部に表示するバナー。
//  残り時間のカウントダウン、発行者、Nice Work! 下書きからの撮影導線、発行者向けの終了ボタンを持つ。

import SwiftUI

struct ActiveChallengeBannerView: View {
    let challenge: ActiveChallenge
    let repository: Repository
    let isEnding: Bool
    let onEnd: () -> Void           //  「終了する」確定時
    let onExpire: () -> Void        //  締め切り到達時（呼び出し側で再取得する）

    @State private var isConfirmingEnd = false
    @State private var didNotifyExpire = false

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            header
            issuerRow
            actionRow
        }
        .padding(16)
        .background(AppTheme.cardBackground)
        .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .stroke(AppTheme.accent.opacity(0.8), lineWidth: 2)
        )
        .confirmationDialog(
            "BeGit Timeを終了しますか？",
            isPresented: $isConfirmingEnd,
            titleVisibility: .visible
        ) {
            Button("終了する", role: .destructive, action: onEnd)
            Button("キャンセル", role: .cancel) {}
        } message: {
            Text("締め切りが今になります。まだ投稿していないメンバーはMissedになり、結果の通知が全員に届きます。")
        }
    }

    // MARK: - Components

    //  見出しと残り時間（1秒ごとに更新）
    private var header: some View {
        TimelineView(.periodic(from: .now, by: 1)) { context in
            let remaining = challenge.remainingSeconds(at: context.date)
            HStack(alignment: .firstTextBaseline) {
                Text("🐙 BeGit Time 進行中")
                    .font(.system(size: 16, weight: .bold, design: .monospaced))
                    .foregroundStyle(AppTheme.accent)

                Spacer()

                Text(remaining > 0 ? "残り \(formatRemaining(remaining))" : "集計中…")
                    .font(.system(size: 14, weight: .semibold, design: .monospaced))
                    .foregroundStyle(remaining > 0 ? AppTheme.softPink : AppTheme.Text.muted)
                    .monospacedDigit()
            }
            .onChange(of: remaining <= 0) { _, expired in
                guard expired, didNotifyExpire == false else { return }
                didNotifyExpire = true
                onExpire()
            }
        }
    }

    //  発行者
    private var issuerRow: some View {
        HStack(spacing: 8) {
            AvatarView(
                member: RepositoryMember(
                    backendUserID: challenge.issuer.userID,
                    login: challenge.issuer.login,
                    avatarURL: challenge.issuer.avatarURL
                ),
                size: 24
            )
            Text(challenge.issuer.login.isEmpty ? "発行者不明" : "\(challenge.issuer.login) が発行")
                .appFont(.body)
                .foregroundStyle(AppTheme.Text.primary)
                .lineLimit(1)

            if challenge.canEnd {
                Text("あなたが発行者")
                    .font(.system(size: 11, weight: .bold, design: .monospaced))
                    .foregroundStyle(.black)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 3)
                    .background(AppTheme.softPink)
                    .clipShape(Capsule())
            }
        }
    }

    //  状態に応じた導線
    @ViewBuilder
    private var actionRow: some View {
        HStack(spacing: 10) {
            if challenge.hasDraftToCapture, let backendID = repository.backendID, let myPost = challenge.myPost {
                //  Nice Work! の下書きあり → Push タップと同じ撮影フローへ
                NavigationLink(
                    value: RepositoryNavigationRoute.notificationNiceWorkDraft(
                        groupId: Int(backendID),
                        draftPostId: Int(myPost.postID),
                        status: myPost.status
                    )
                ) {
                    actionLabel("撮影して投稿", systemImage: "camera.fill", fill: AppTheme.accent)
                }
                .buttonStyle(.plain)
            } else if challenge.hasPosted {
                statusBadge
            } else {
                Text("GitHubにpushするとNice Work!が届き、ここから投稿できます")
                    .appFont(.caption)
                    .foregroundStyle(AppTheme.Text.muted)
                    .lineSpacing(2)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }

            if challenge.canEnd {
                Button {
                    isConfirmingEnd = true
                } label: {
                    actionLabel(isEnding ? "終了中…" : "終了する", systemImage: "stop.fill", fill: AppTheme.softPink)
                        .frame(maxWidth: 120)
                }
                .buttonStyle(.plain)
                .disabled(isEnding)
                .accessibilityLabel("BeGit Timeを終了する")
            }
        }
    }

    //  投稿済みバッジ（On Time / Late）
    private var statusBadge: some View {
        let isLate = challenge.myPost?.status == "late"
        return HStack(spacing: 6) {
            Image(systemName: isLate ? "clock.badge.exclamationmark" : "checkmark.seal.fill")
                .font(.system(size: 14, weight: .bold))
            Text(isLate ? "投稿済み · Late" : "投稿済み · On Time")
                .font(.system(size: 13, weight: .bold, design: .monospaced))
        }
        .foregroundStyle(isLate ? AppTheme.softPink : AppTheme.accent)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func actionLabel(_ title: String, systemImage: String, fill: Color) -> some View {
        HStack(spacing: 6) {
            Image(systemName: systemImage)
                .font(.system(size: 14, weight: .bold))
            Text(title)
                .font(.system(size: 14, weight: .bold, design: .monospaced))
        }
        .foregroundStyle(.black)
        .frame(maxWidth: .infinity)
        .frame(height: 40)
        .background(fill)
        .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
    }

    //  mm:ss（1時間以上は h:mm:ss）
    private func formatRemaining(_ seconds: TimeInterval) -> String {
        let total = Int(seconds.rounded(.up))
        let hours = total / 3600
        let minutes = (total % 3600) / 60
        let secs = total % 60
        if hours > 0 {
            return String(format: "%d:%02d:%02d", hours, minutes, secs)
        }
        return String(format: "%02d:%02d", minutes, secs)
    }
}
