-- Points are an append-only ledger. The unique key is the award identity, not
-- the post ID: a photo and a GitHub event can confirm the same report.
ALTER TABLE groups ADD COLUMN completed_at TEXT;

CREATE TABLE notification_members (
  notification_id INTEGER NOT NULL REFERENCES notifications(id),
  user_id INTEGER NOT NULL REFERENCES users(id),
  PRIMARY KEY (notification_id, user_id)
);

CREATE TABLE point_awards (
  group_id INTEGER NOT NULL REFERENCES groups(id),
  notification_id INTEGER NOT NULL REFERENCES notifications(id),
  user_id INTEGER NOT NULL REFERENCES users(id),
  kind TEXT NOT NULL CHECK (kind IN ('report', 'all_on_time_bonus')),
  points INTEGER NOT NULL CHECK (points = 10),
  awarded_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (notification_id, user_id, kind)
);
CREATE INDEX point_awards_group_user ON point_awards(group_id, user_id);

CREATE TABLE final_result_members (
  group_id INTEGER NOT NULL REFERENCES groups(id),
  user_id INTEGER NOT NULL REFERENCES users(id),
  login TEXT NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  report_points INTEGER NOT NULL,
  bonus_points INTEGER NOT NULL,
  PRIMARY KEY (group_id, user_id)
);

-- Every notification captures its audience in the same DB statement that
-- creates it, including scheduled notifications.
CREATE TRIGGER notification_capture_members AFTER INSERT ON notifications
BEGIN
  INSERT INTO notification_members(notification_id, user_id)
  SELECT NEW.id, user_id FROM group_members
  WHERE group_id = (SELECT group_id FROM sprints WHERE id = NEW.sprint_id) AND role IN ('owner', 'poster')
END;

CREATE TRIGGER notification_after_completion BEFORE INSERT ON notifications
WHEN (SELECT completed_at FROM groups WHERE id =
  (SELECT group_id FROM sprints WHERE id = NEW.sprint_id)) IS NOT NULL
BEGIN
  SELECT RAISE(ABORT, 'project completed');
END;

-- Confirmed reports only. The original report timestamp is kept even if a
-- photo is added later. Drafts and synthetic missed posts never earn points.
CREATE TRIGGER post_award_insert AFTER INSERT ON posts
WHEN NEW.notification_id IS NOT NULL AND NEW.is_draft = 0
  AND COALESCE(NEW.status, '') != 'missed'
BEGIN
  INSERT OR IGNORE INTO point_awards(group_id, notification_id, user_id, kind, points)
  SELECT NEW.group_id, NEW.notification_id, NEW.user_id, 'report', 10
  WHERE (SELECT completed_at FROM groups WHERE id = NEW.group_id) IS NULL;
END;

CREATE TRIGGER post_award_confirm AFTER UPDATE OF is_draft ON posts
WHEN OLD.is_draft = 1 AND NEW.is_draft = 0
  AND NEW.notification_id IS NOT NULL AND COALESCE(NEW.status, '') != 'missed'
BEGIN
  INSERT OR IGNORE INTO point_awards(group_id, notification_id, user_id, kind, points)
  SELECT NEW.group_id, NEW.notification_id, NEW.user_id, 'report', 10
  WHERE (SELECT completed_at FROM groups WHERE id = NEW.group_id) IS NULL;
END;

-- Completion and the per-member result snapshot happen atomically. The
-- completed_at transition only occurs once, so retries preserve the result.
CREATE TRIGGER group_capture_final_result AFTER UPDATE OF completed_at ON groups
WHEN OLD.completed_at IS NULL AND NEW.completed_at IS NOT NULL
BEGIN
  INSERT INTO final_result_members(group_id, user_id, login, avatar_url, report_points, bonus_points)
  SELECT NEW.id, participants.user_id, u.github_login, COALESCE(u.avatar_url, ''),
    COALESCE(SUM(CASE WHEN pa.kind = 'report' THEN pa.points ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN pa.kind = 'all_on_time_bonus' THEN pa.points ELSE 0 END), 0)
  FROM (
    SELECT user_id FROM group_members WHERE group_id = NEW.id
    UNION
    SELECT user_id FROM point_awards WHERE group_id = NEW.id
  ) participants
  JOIN users u ON u.id = participants.user_id
  LEFT JOIN point_awards pa ON pa.group_id = NEW.id AND pa.user_id = participants.user_id
  GROUP BY participants.user_id;
END;

-- Preserve eligible reports created before this migration. Historical audience
-- membership is unavailable, so old notifications use the current membership.
INSERT OR IGNORE INTO notification_members(notification_id, user_id)
SELECT n.id, gm.user_id FROM notifications n
JOIN sprints s ON s.id = n.sprint_id
JOIN group_members gm ON gm.group_id = s.group_id
WHERE gm.role IN ('owner', 'poster');

INSERT OR IGNORE INTO point_awards(group_id, notification_id, user_id, kind, points)
SELECT p.group_id, p.notification_id, p.user_id, 'report', 10
FROM posts p
WHERE p.notification_id IS NOT NULL AND p.is_draft = 0
  AND COALESCE(p.status, '') != 'missed'
  AND (
    p.post_type != 'memo'
    OR EXISTS (SELECT 1 FROM photos ph WHERE ph.post_id = p.id)
  );
