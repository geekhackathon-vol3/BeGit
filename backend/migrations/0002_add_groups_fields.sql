-- groups テーブルに name と avatar_url カラムを追加する
-- Req 2.1, 2.2: グループ一覧・作成で name と avatar_url が必要
ALTER TABLE groups ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE groups ADD COLUMN avatar_url TEXT;
