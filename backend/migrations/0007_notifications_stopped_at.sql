-- 手動停止したBeGit Timeの終了時刻を保持する。
ALTER TABLE notifications ADD COLUMN stopped_at TEXT;
