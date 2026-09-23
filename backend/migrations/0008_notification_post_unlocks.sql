-- BeGit Time への投稿達成を、表示用の posts とは別に保持する。
-- 投稿を削除しても「投稿して表示」の解除状態は維持される。
CREATE TABLE IF NOT EXISTS notification_post_unlocks (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  notification_id INTEGER NOT NULL REFERENCES notifications(id),
  user_id         INTEGER NOT NULL REFERENCES users(id),
  group_id        INTEGER NOT NULL REFERENCES groups(id),
  created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
  UNIQUE(notification_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_notification_post_unlocks_user_group_notification
  ON notification_post_unlocks(user_id, group_id, notification_id);

-- 既に確定済みの投稿は、リリース後も従来どおり表示を解除したままにする。
INSERT OR IGNORE INTO notification_post_unlocks (notification_id, user_id, group_id)
SELECT notification_id, user_id, group_id
FROM posts
WHERE notification_id IS NOT NULL
  AND is_draft = 0
  AND (status IS NULL OR status != 'missed');
