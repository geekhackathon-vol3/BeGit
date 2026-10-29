-- Persist the selected game mode and the audience request on the notification.
-- An empty audience keeps backward compatibility with old clients and scheduled
-- BeGit Times by selecting every member who can post.
ALTER TABLE notifications ADD COLUMN game_mode TEXT NOT NULL DEFAULT 'everyone'
  CHECK (game_mode IN ('everyone', 'spotlight'));
ALTER TABLE notifications ADD COLUMN audience_user_ids TEXT NOT NULL DEFAULT '[]';

-- Replace the #120 all-member capture with an atomic selected-member capture.
-- Membership and role are checked again in SQLite, so arbitrary user IDs sent by
-- a client can never enter the frozen notification audience.
DROP TRIGGER notification_capture_members;
CREATE TRIGGER notification_capture_members AFTER INSERT ON notifications
BEGIN
  INSERT INTO notification_members(notification_id, user_id)
  SELECT NEW.id, gm.user_id
  FROM group_members gm
  WHERE gm.group_id = (SELECT group_id FROM sprints WHERE id = NEW.sprint_id)
    AND gm.role IN ('owner', 'poster')
    AND (
      json_array_length(NEW.audience_user_ids) = 0
      OR gm.user_id IN (
        SELECT CAST(value AS INTEGER) FROM json_each(NEW.audience_user_ids)
      )
    );
END;

-- One immutable result per BeGit Time. Absence means that the challenge is
-- still pending. Existing notifications are treated as everyone mode by the
-- notifications.game_mode default.
CREATE TABLE notification_game_results (
  notification_id INTEGER PRIMARY KEY REFERENCES notifications(id),
  game_mode TEXT NOT NULL CHECK (game_mode IN ('everyone', 'spotlight')),
  outcome TEXT NOT NULL CHECK (outcome IN ('succeeded', 'failed', 'cancelled')),
  bonus_user_id INTEGER REFERENCES users(id),
  bonus_points INTEGER NOT NULL DEFAULT 0 CHECK (bonus_points IN (0, 10)),
  finalized_at TEXT NOT NULL DEFAULT (datetime('now')),
  CHECK (
    (outcome = 'succeeded' AND bonus_user_id IS NOT NULL AND bonus_points = 10)
    OR (outcome != 'succeeded' AND bonus_user_id IS NULL AND bonus_points = 0)
  )
);

-- Keep #120's point ledger schema stable. The notification result records the
-- actual mode, while the existing bonus award kind remains the aggregate bonus
-- bucket used by final_result_members.
CREATE TRIGGER notification_game_result_award AFTER INSERT ON notification_game_results
WHEN NEW.outcome = 'succeeded'
BEGIN
  INSERT OR IGNORE INTO point_awards(group_id, notification_id, user_id, kind, points)
  SELECT s.group_id, NEW.notification_id, NEW.bonus_user_id, 'all_on_time_bonus', NEW.bonus_points
  FROM notifications n
  JOIN sprints s ON s.id = n.sprint_id
  JOIN groups g ON g.id = s.group_id
  WHERE n.id = NEW.notification_id AND g.completed_at IS NULL;
END;
