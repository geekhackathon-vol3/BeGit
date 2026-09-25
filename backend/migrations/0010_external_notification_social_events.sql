-- 投稿・リアクション・コメントの外部通知購読を追加する。
-- 既存チャンネルにも新しい通知種別を自動で購読させ、通知漏れを防ぐ。
DROP INDEX IF EXISTS idx_notification_channel_subscriptions_event;

ALTER TABLE notification_channel_subscriptions RENAME TO notification_channel_subscriptions_old;

CREATE TABLE notification_channel_subscriptions (
  channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL CHECK (event_type IN (
    'begit_time', 'challenge_end', 'sprint_reminder', 'sprint_end', 'sprint_start',
    'nice_work', 'reaction', 'comment'
  )),
  PRIMARY KEY(channel_id, event_type)
);

INSERT INTO notification_channel_subscriptions (channel_id, event_type)
SELECT channel_id, event_type FROM notification_channel_subscriptions_old;

INSERT OR IGNORE INTO notification_channel_subscriptions (channel_id, event_type)
SELECT id, event_type
FROM notification_channels
CROSS JOIN (
  SELECT 'nice_work' AS event_type UNION ALL
  SELECT 'reaction' UNION ALL
  SELECT 'comment'
)
WHERE enabled = 1;

DROP TABLE notification_channel_subscriptions_old;

CREATE INDEX idx_notification_channel_subscriptions_event
  ON notification_channel_subscriptions(event_type, channel_id);
