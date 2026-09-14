-- 公開リポジトリの表示専用登録を識別する。
ALTER TABLE groups ADD COLUMN read_only INTEGER NOT NULL DEFAULT 0;
