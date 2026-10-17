-- BeGit内の権限を owner / poster / viewer の3段階に分ける。
-- 既存memberは従来どおり投稿可能として扱う。
UPDATE group_members SET role = 'poster' WHERE role = 'member';
