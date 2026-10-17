-- グループごとの BeGit Time 自動開催設定。
-- Cron Trigger は毎分1本だけ動かし、next_run_at が到来した設定を D1 から取得する。
CREATE TABLE IF NOT EXISTS begit_time_schedules (
  group_id       INTEGER PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
  enabled        INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
  local_time     TEXT    NOT NULL DEFAULT '09:00',
  timezone       TEXT    NOT NULL DEFAULT 'Asia/Tokyo',
  weekdays       TEXT    NOT NULL DEFAULT '1,2,3,4,5',
  next_run_at    TEXT,
  updated_by     INTEGER NOT NULL REFERENCES users(id),
  created_at     TEXT    NOT NULL DEFAULT (datetime('now')),
  updated_at     TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_begit_time_schedules_due
  ON begit_time_schedules(enabled, next_run_at);

-- 手動開催と自動開催を区別する。sent_by は既存FKとの互換性と管理主体を表すため、
-- 自動開催ではグループオーナーを保存する。
ALTER TABLE notifications ADD COLUMN source TEXT NOT NULL DEFAULT 'manual'
  CHECK (source IN ('manual', 'scheduled'));

CREATE INDEX IF NOT EXISTS idx_notifications_source
  ON notifications(source, sent_at);
