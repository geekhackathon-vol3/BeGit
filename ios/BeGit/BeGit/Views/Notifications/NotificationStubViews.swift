//  NotificationStubViews.swift
//  #55（通知ルーティング基盤）の遷移先スタブ。type 別に正しい画面へ着地することの確認用。
//  中身は後続 issue が実装する:
//    - ① memo 投稿 / ② Nice Work 確定フロー → #56
//    - ⑦ リアクション・コメント → 投稿詳細 → #57

import SwiftUI

//  スタブ共通の見た目。画面名と受け取った ID を並べるだけ。
private struct NotificationStubScaffold: View {
    let title: String
    let todo: String
    let fields: [(label: String, value: String)]

    var body: some View {
        ZStack {
            AppTheme.background.ignoresSafeArea()
            VStack(alignment: .leading, spacing: 16) {
                Text(title)
                    .font(.system(size: 22, weight: .black, design: .monospaced))
                    .foregroundStyle(.white)

                VStack(alignment: .leading, spacing: 6) {
                    ForEach(fields, id: \.label) { field in
                        Text("\(field.label): \(field.value)")
                            .font(.system(size: 13, weight: .semibold, design: .monospaced))
                            .foregroundStyle(.white.opacity(0.78))
                    }
                }

                Text(todo)
                    .font(.system(size: 12, weight: .regular, design: .monospaced))
                    .foregroundStyle(AppTheme.accent)

                Spacer()
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(24)
        }
        .navigationBarTitleDisplayMode(.inline)
    }
}

//  ① begit_time → memo 投稿 UI
struct NotificationPostCreationStubView: View {
    let groupId: Int
    let notificationId: Int?

    var body: some View {
        NotificationStubScaffold(
            title: "BeGit Time! 投稿作成",
            todo: "TODO: #56 で memo 投稿 UI を実装",
            fields: [
                ("group_id", "\(groupId)"),
                ("notification_id", notificationId.map(String.init) ?? "-")
            ]
        )
    }
}

//  ② nice_work → 下書きプレフィル → 撮影 → 確定
struct NotificationNiceWorkDraftStubView: View {
    let groupId: Int
    let draftPostId: Int
    let status: String?

    var body: some View {
        NotificationStubScaffold(
            title: "Nice Work! 下書き確定",
            todo: "TODO: #56 で下書き取得→撮影→確定フローを実装",
            fields: [
                ("group_id", "\(groupId)"),
                ("draft_post_id", "\(draftPostId)"),
                ("status", status ?? "-")
            ]
        )
    }
}

//  ③ challenge_end → チャレンジ結果画面
struct NotificationChallengeResultStubView: View {
    let groupId: Int
    let notificationId: Int

    var body: some View {
        NotificationStubScaffold(
            title: "チャレンジ結果",
            todo: "TODO: GET /groups/:id/notifications/:nid で結果を表示",
            fields: [
                ("group_id", "\(groupId)"),
                ("notification_id", "\(notificationId)")
            ]
        )
    }
}

//  ④⑥ sprint_reminder / sprint_start → スプリント概要画面
struct NotificationSprintOverviewStubView: View {
    let groupId: Int
    let sprintId: Int

    var body: some View {
        NotificationStubScaffold(
            title: "スプリント概要",
            todo: "TODO: GET /groups/:id で概要を表示",
            fields: [
                ("group_id", "\(groupId)"),
                ("sprint_id", "\(sprintId)")
            ]
        )
    }
}

//  ⑤ sprint_end → スプリント結果画面
struct NotificationSprintResultStubView: View {
    let groupId: Int
    let sprintId: Int

    var body: some View {
        NotificationStubScaffold(
            title: "スプリント結果",
            todo: "TODO: GET /groups/:id + GET …/posts で結果を表示",
            fields: [
                ("group_id", "\(groupId)"),
                ("sprint_id", "\(sprintId)")
            ]
        )
    }
}

//  ⑦ reaction / comment → 投稿詳細
struct NotificationPostDetailStubView: View {
    let groupId: Int
    let postId: Int
    let kind: NotificationSocialKind

    var body: some View {
        NotificationStubScaffold(
            title: kind == .reaction ? "投稿詳細（リアクション）" : "投稿詳細（コメント）",
            todo: "TODO: #57 で投稿詳細＋反応一覧を実装",
            fields: [
                ("group_id", "\(groupId)"),
                ("post_id", "\(postId)")
            ]
        )
    }
}
