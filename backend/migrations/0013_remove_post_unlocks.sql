-- 旧 Nice Work の GitHub 下書きは活動検知済みなので確定済みにする。
UPDATE posts SET is_draft = 0
WHERE notification_id IS NOT NULL AND is_draft = 1
  AND post_type IN ('commit', 'pull_request', 'issue', 'review');

-- 閲覧制限の廃止により表示解除記録は不要。
DROP TABLE IF EXISTS notification_post_unlocks;
