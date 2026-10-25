"""SQLite checks for the point ledger and atomic completion snapshot.

Run with: python3 scripts/test_points_migration.py
"""

import glob
import re
import sqlite3
import unittest


def finalize_bonuses_sql():
    with open("internal/repository/points_repository.go", encoding="utf-8") as source:
        code = source.read()
    return re.search(r"func \(r \*PointsRepository\) FinalizeBonuses.*?Exec\(ctx, `(.*?)`, \[\]interface", code, re.S).group(1)


class PointsMigrationTest(unittest.TestCase):
    def setUp(self):
        self.db = sqlite3.connect(":memory:")
        for path in sorted(glob.glob("migrations/*.sql")):
            with open(path, encoding="utf-8") as source:
                self.db.executescript(source.read())
        db = self.db
        db.execute("INSERT INTO users(id, github_id, github_login, encrypted_access_token) VALUES (1, 1, 'owner', 'a'), (2, 2, 'member', 'b')")
        db.execute("INSERT INTO groups(id, repo_full_name, owner_user_id) VALUES (1, 'owner/repo', 1)")
        db.execute("INSERT INTO group_members(group_id, user_id, role) VALUES (1, 1, 'owner'), (1, 2, 'poster')")
        db.execute("INSERT INTO sprints(id, group_id, ends_at) VALUES (1, 1, datetime('now', '+7 days'))")

    def awards(self):
        return self.db.execute("SELECT user_id, kind, points FROM point_awards ORDER BY user_id, kind").fetchall()

    def test_report_is_awarded_once_after_photo_confirmation(self):
        db = self.db
        db.execute("INSERT INTO notifications(id, sprint_id, sent_by) VALUES (1, 1, 1)")
        self.assertEqual(db.execute("SELECT count(*) FROM notification_members").fetchone()[0], 2)
        db.execute("INSERT INTO posts(id, notification_id, user_id, group_id, is_draft) VALUES (1, 1, 2, 1, 1)")
        self.assertEqual(self.awards(), [])
        db.execute("UPDATE posts SET is_draft = 0, status = 'on_time' WHERE id = 1")
        db.execute("UPDATE posts SET is_draft = 0 WHERE id = 1")
        self.assertEqual(self.awards(), [(2, 'report', 10)])

    def test_completion_snapshots_result_and_blocks_more_points(self):
        db = self.db
        db.execute("INSERT INTO notifications(id, sprint_id, sent_by, sent_at) VALUES (1, 1, 1, datetime('now', '-2 hours'))")
        db.execute("INSERT INTO posts(notification_id, user_id, group_id, status, is_draft) VALUES (1, 2, 1, 'late', 0)")
        db.execute("UPDATE groups SET completed_at = datetime('now') WHERE id = 1 AND completed_at IS NULL")
        db.execute("UPDATE groups SET completed_at = datetime('now') WHERE id = 1 AND completed_at IS NULL")
        self.assertEqual(db.execute("SELECT user_id, report_points, bonus_points FROM final_result_members ORDER BY user_id").fetchall(), [(1, 0, 0), (2, 10, 0)])
        with self.assertRaises(sqlite3.IntegrityError):
            db.execute("INSERT INTO notifications(sprint_id, sent_by) VALUES (1, 1)")
        db.execute("INSERT INTO posts(notification_id, user_id, group_id, status, is_draft) VALUES (1, 1, 1, 'late', 0)")
        self.assertEqual(self.awards(), [(2, 'report', 10)])

    def test_bonus_requires_every_captured_member_and_is_idempotent(self):
        db = self.db
        db.execute("INSERT INTO notifications(id, sprint_id, sent_by, sent_at) VALUES (1, 1, 1, datetime('now'))")
        # A new member after issuance is not part of this challenge.
        db.execute("INSERT INTO users(id, github_id, github_login, encrypted_access_token) VALUES (3, 3, 'later', 'c')")
        db.execute("INSERT INTO group_members(group_id, user_id, role) VALUES (1, 3, 'poster')")
        bonus_sql = finalize_bonuses_sql()
        db.execute("INSERT INTO posts(notification_id, user_id, group_id, status, is_draft, created_at) VALUES (1, 1, 1, 'on_time', 0, datetime('now', '-90 minutes'))")
        db.execute(bonus_sql, (1, 1, 1))
        self.assertNotIn((1, 'all_on_time_bonus', 10), self.awards())
        db.execute("INSERT INTO posts(notification_id, user_id, group_id, status, is_draft, created_at) VALUES (1, 2, 1, 'on_time', 0, datetime('now', '-90 minutes'))")
        db.execute("UPDATE notifications SET sent_at = datetime('now', '-2 hours') WHERE id = 1")
        db.execute(bonus_sql, (1, 1, 1))
        db.execute(bonus_sql, (1, 1, 1))
        self.assertEqual(self.awards().count((1, 'all_on_time_bonus', 10)), 1)
        db.execute("UPDATE groups SET completed_at = datetime('now') WHERE id = 1 AND completed_at IS NULL")
        self.assertEqual(db.execute("SELECT report_points, bonus_points FROM final_result_members WHERE user_id = 1").fetchone(), (10, 10))

    def test_selected_audience_and_spotlight_result_are_frozen_and_idempotent(self):
        db = self.db
        db.execute("INSERT INTO users(id, github_id, github_login, encrypted_access_token) VALUES (3, 3, 'excluded', 'c')")
        db.execute("INSERT INTO group_members(group_id, user_id, role) VALUES (1, 3, 'poster')")
        db.execute("INSERT INTO notifications(id, sprint_id, sent_by, sent_at, game_mode, audience_user_ids) VALUES (1, 1, 1, datetime('now', '-2 hours'), 'spotlight', '[1,2]')")
        self.assertEqual(db.execute("SELECT user_id FROM notification_members WHERE notification_id = 1 ORDER BY user_id").fetchall(), [(1,), (2,)])
        db.execute("DELETE FROM group_members WHERE user_id = 2")
        db.execute("INSERT INTO posts(notification_id, user_id, group_id, status, is_draft, created_at) VALUES (1, 1, 1, 'on_time', 0, datetime('now', '-90 minutes'))")
        bonus_sql = finalize_bonuses_sql()
        db.execute(bonus_sql, (1, 1, 1))
        db.execute(bonus_sql, (1, 1, 1))
        self.assertEqual(db.execute("SELECT game_mode, outcome, bonus_user_id, bonus_points FROM notification_game_results").fetchone(), ('spotlight', 'succeeded', 1, 10))
        self.assertEqual(self.awards().count((1, 'all_on_time_bonus', 10)), 1)

    def test_spotlight_fails_when_another_target_reports(self):
        db = self.db
        db.execute("INSERT INTO notifications(id, sprint_id, sent_by, sent_at, game_mode, audience_user_ids) VALUES (1, 1, 1, datetime('now', '-2 hours'), 'spotlight', '[1,2]')")
        db.execute("INSERT INTO posts(notification_id, user_id, group_id, status, is_draft, created_at) VALUES (1, 1, 1, 'on_time', 0, datetime('now', '-90 minutes'))")
        db.execute("INSERT INTO posts(notification_id, user_id, group_id, status, is_draft, created_at) VALUES (1, 2, 1, 'on_time', 0, datetime('now', '-90 minutes'))")
        bonus_sql = finalize_bonuses_sql()
        db.execute(bonus_sql, (1, 1, 1))
        self.assertEqual(db.execute("SELECT game_mode, outcome, bonus_points FROM notification_game_results").fetchone(), ('spotlight', 'failed', 0))
        self.assertNotIn((1, 'all_on_time_bonus', 10), self.awards())

    def test_stopped_notification_is_cancelled_once(self):
        db = self.db
        db.execute("INSERT INTO notifications(id, sprint_id, sent_by, sent_at, stopped_at, game_mode) VALUES (1, 1, 1, datetime('now'), datetime('now'), 'everyone')")
        bonus_sql = finalize_bonuses_sql()
        db.execute(bonus_sql, (1, 1, 1))
        db.execute(bonus_sql, (1, 1, 1))
        self.assertEqual(db.execute("SELECT game_mode, outcome, bonus_user_id, bonus_points FROM notification_game_results").fetchone(), ('everyone', 'cancelled', None, 0))

    def test_cron_only_checks_recent_challenges_but_completion_checks_all(self):
        db = self.db
        bonus_sql = finalize_bonuses_sql()
        db.execute("INSERT INTO notifications(id, sprint_id, sent_by, sent_at) VALUES (1, 1, 1, datetime('now', '-30 hours'))")
        db.execute("INSERT INTO posts(notification_id, user_id, group_id, status, is_draft, created_at) VALUES (1, 1, 1, 'on_time', 0, datetime('now', '-30 hours')), (1, 2, 1, 'on_time', 0, datetime('now', '-30 hours'))")
        db.execute(bonus_sql, (0, 0, 0))
        self.assertNotIn((1, 'all_on_time_bonus', 10), self.awards())
        db.execute(bonus_sql, (1, 1, 1))
        self.assertIn((1, 'all_on_time_bonus', 10), self.awards())

    # 手元の SQLite の実行計画での目安。D1 の実行計画を保証するものではない。
    def test_cron_bonus_check_uses_sent_at_index(self):
        plan = self.db.execute("EXPLAIN QUERY PLAN " + finalize_bonuses_sql(), (0, 0, 0)).fetchall()
        self.assertTrue(any("idx_notifications_sent_at" in row[-1] for row in plan), plan)


if __name__ == "__main__":
    unittest.main()
