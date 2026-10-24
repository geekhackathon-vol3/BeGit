import SwiftUI

struct ProjectFinalResultView: View {
    let repositoryName: String
    let result: ProjectFinalResult

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                    Text("作品完成リザルト")
                        .font(.custom("Bitcount", size: 32))
                        .foregroundStyle(AppTheme.accent)
                    Text(repositoryName)
                        .font(.headline)
                        .foregroundStyle(.white)

                    HStack {
                        Text("TEAM TOTAL")
                        Spacer()
                        Text("\(result.teamTotal) pt")
                    }
                    .font(.system(size: 20, weight: .bold, design: .monospaced))
                    .foregroundStyle(AppTheme.accent)
                    .padding(18)
                    .background(AppTheme.fieldBackground)
                    .clipShape(RoundedRectangle(cornerRadius: 14))

                    if result.modeResults.isEmpty == false {
                        VStack(alignment: .leading, spacing: 12) {
                            Text("GAME RESULTS")
                                .font(.system(size: 14, weight: .bold, design: .monospaced))
                                .foregroundStyle(AppTheme.sectionPink)
                            ForEach(result.modeResults) { mode in
                                HStack(spacing: 12) {
                                    Image(systemName: mode.gameMode.systemImage)
                                        .foregroundStyle(AppTheme.accent)
                                    VStack(alignment: .leading, spacing: 3) {
                                        Text(mode.gameMode.title)
                                            .font(.system(size: 14, weight: .bold, design: .monospaced))
                                        Text("\(mode.successes)/\(mode.attempts) 達成")
                                            .font(.system(size: 12, design: .monospaced))
                                            .foregroundStyle(AppTheme.Text.medium)
                                        if mode.cancelled > 0 {
                                            Text("途中終了 \(mode.cancelled)回")
                                                .font(.system(size: 11, design: .monospaced))
                                                .foregroundStyle(AppTheme.Text.muted)
                                        }
                                    }
                                    Spacer()
                                    Text("+\(mode.bonusPoints) pt")
                                        .font(.system(size: 14, weight: .bold, design: .monospaced))
                                        .foregroundStyle(AppTheme.accent)
                                }
                            }
                        }
                        .foregroundStyle(.white)
                        .padding(16)
                        .background(AppTheme.fieldBackground)
                        .clipShape(RoundedRectangle(cornerRadius: 12))
                    }

                    ForEach(result.members) { member in
                        VStack(alignment: .leading, spacing: 12) {
                            HStack {
                                Text("@\(member.login)")
                                Spacer()
                                Text("\(member.totalPoints) pt")
                                    .foregroundStyle(AppTheme.accent)
                            }
                            .font(.system(size: 16, weight: .bold, design: .monospaced))
                            HStack {
                                Text("報告 \(member.reportPoints) pt")
                                Spacer()
                                Text("モードボーナス \(member.bonusPoints) pt")
                            }
                            .font(.system(size: 12, design: .monospaced))
                            .foregroundStyle(AppTheme.Text.medium)
                        }
                        .foregroundStyle(.white)
                        .padding(16)
                        .background(AppTheme.fieldBackground)
                        .clipShape(RoundedRectangle(cornerRadius: 12))
                    }
                }
                .padding(20)
            }
            .background(AppTheme.background.ignoresSafeArea())
            .navigationTitle("Final Result")
            .navigationBarTitleDisplayMode(.inline)
        }
    }
}
