-- Slack / Discord への外部通知チャネルと、非同期配信ジョブ。
-- Webhook URL はアプリケーション層で AES-GCM 暗号化して保存する。

CREATE TABLE IF NOT EXISTS notification_channels (
  id                    INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id              INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  platform              TEXT NOT NULL CHECK (platform IN ('slack', 'discord')),
  display_name          TEXT NOT NULL,
  encrypted_webhook_url TEXT NOT NULL,
  enabled               INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
  created_by            INTEGER NOT NULL REFERENCES users(id),
  created_at            TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at            TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(group_id, platform, display_name)
);

CREATE INDEX IF NOT EXISTS idx_notification_channels_group
  ON notification_channels(group_id, enabled);

CREATE TABLE IF NOT EXISTS notification_channel_subscriptions (
  channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL CHECK (event_type IN (
    'begit_time',
    'challenge_end',
    'sprint_reminder',
    'sprint_end',
    'sprint_start'
  )),
  PRIMARY KEY(channel_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_notification_channel_subscriptions_event
  ON notification_channel_subscriptions(event_type, channel_id);

CREATE TABLE IF NOT EXISTS notification_delivery_jobs (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  event_key     TEXT NOT NULL,
  channel_id    INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
  event_type    TEXT NOT NULL,
  payload_json  TEXT NOT NULL,
  status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delivered', 'failed')),
  attempt_count INTEGER NOT NULL DEFAULT 0,
  last_error    TEXT,
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  delivered_at TEXT,
  UNIQUE(event_key, channel_id)
);

CREATE INDEX IF NOT EXISTS idx_notification_delivery_jobs_status
  ON notification_delivery_jobs(status, created_at);
