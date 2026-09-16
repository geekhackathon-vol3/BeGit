-- ============================================================
-- notifications から UNIQUE(sprint_id, sent_by) を撤去する
--
-- 「1スプリント1人1回」を設定（BEGIT_TIME_ALLOW_MULTIPLE_PER_SPRINT）で解除できるようにするため、
-- 判定を DB 制約からサービス層（CreateIfNoActive の INSERT 条件）へ移す。
-- 設定が未設定（本番）の間は、INSERT 条件で従来どおり1スプリント1人1回が保たれる。
--
-- SQLite は ALTER で UNIQUE 制約を削除できないため、テーブルを作り直す。
-- posts.notification_id が notifications(id) を参照しているので、外部キー検査をコミット時まで遅延させ、
-- id を保持したまま移し替える（D1 では PRAGMA foreign_keys=OFF は使えない）。
-- 注意: 別名の新テーブルを RENAME する方式だと、DROP で数えられた参照切れが解消されずコミット時に失敗する。
--       そのため「退避 → DROP → 同名で再作成 → 戻す」の順にし、戻す INSERT で参照切れを解消させる。
-- ============================================================

PRAGMA defer_foreign_keys = true;

-- 1. 退避（外部キーを持たない一時テーブル）
CREATE TABLE notifications_backup_0006 (
  id        INTEGER,
  sprint_id INTEGER,
  sent_by   INTEGER,
  message   TEXT,
  sent_at   TEXT
);
INSERT INTO notifications_backup_0006 (id, sprint_id, sent_by, message, sent_at)
SELECT id, sprint_id, sent_by, message, sent_at FROM notifications;

-- AUTOINCREMENT の採番位置も退避する（DROP で消え、戻すと「現存する最大 id」まで巻き戻るため）。
-- 削除済み通知の id が再利用されると notification_deliveries.ref_id の送信済み記録と衝突する。
CREATE TABLE notifications_seq_0006 (seq INTEGER);
INSERT INTO notifications_seq_0006 (seq)
SELECT seq FROM sqlite_sequence WHERE name = 'notifications';

-- 2. UNIQUE 付きの旧テーブルを削除
DROP TABLE notifications;

-- 3. UNIQUE(sprint_id, sent_by) を除いた定義で同名に再作成（他は 0001 と同じ）
CREATE TABLE notifications (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  sprint_id INTEGER NOT NULL REFERENCES sprints(id),
  sent_by   INTEGER NOT NULL REFERENCES users(id),
  message   TEXT    NOT NULL DEFAULT '今なに作ってる？',
  sent_at   TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- 4. id を保持して戻す（posts.notification_id の参照を維持）
INSERT INTO notifications (id, sprint_id, sent_by, message, sent_at)
SELECT id, sprint_id, sent_by, message, sent_at FROM notifications_backup_0006;

-- 採番位置を元に戻す（戻した行が無く sqlite_sequence に行が無い場合も考慮）
UPDATE sqlite_sequence
SET seq = MAX(seq, (SELECT seq FROM notifications_seq_0006))
WHERE name = 'notifications' AND EXISTS (SELECT 1 FROM notifications_seq_0006);
INSERT INTO sqlite_sequence (name, seq)
SELECT 'notifications', seq FROM notifications_seq_0006
WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = 'notifications');

DROP TABLE notifications_backup_0006;
DROP TABLE notifications_seq_0006;
