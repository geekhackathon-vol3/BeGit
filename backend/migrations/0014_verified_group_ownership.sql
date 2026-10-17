-- GitHub上の管理権限を確認済みのBeGit ownerかどうかを保持する。
-- 既存の表示専用グループは、最初の登録者がGitHub ownerとは限らないため未確認へ戻す。
ALTER TABLE groups ADD COLUMN owner_verified INTEGER NOT NULL DEFAULT 1;
ALTER TABLE groups ADD COLUMN owner_claimed_at TEXT;

UPDATE groups
SET owner_verified = 0,
    owner_claimed_at = NULL
WHERE read_only = 1;

UPDATE groups
SET owner_claimed_at = created_at
WHERE owner_verified = 1;

UPDATE group_members
SET role = 'viewer'
WHERE role = 'owner'
  AND group_id IN (
    SELECT id FROM groups WHERE owner_verified = 0
  );
