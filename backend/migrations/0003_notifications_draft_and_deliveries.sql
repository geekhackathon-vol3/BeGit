-- ============================================================
-- begit-notifications: draft 状態 + 送信済み冪等テーブル
--
-- 変更点:
--   1. posts に is_draft 列を追加（写真の有無と独立に draft を管理）
--      既存行はデフォルト 0（＝非 draft）で後方互換。
--   2. notification_deliveries を新規作成（Cron 通知③④⑤⑥の送信済み冪等）
--      UNIQUE(kind, ref_id) への INSERT 成功時のみ FCM 送信する。
--
-- 適用: wrangler d1 migrations apply begit-db
--   ローカル/dev で適用後、posts.is_draft 列と notification_deliveries
--   テーブルがスキーマに反映されることを確認する。
-- ============================================================

-- posts に draft フラグを追加（写真有無と独立。is_draft=1 はフィード除外）
ALTER TABLE posts ADD COLUMN is_draft INTEGER NOT NULL DEFAULT 0;

-- Cron 通知の送信済み冪等（③ challenge_end / ④ sprint_reminder / ⑤ sprint_end / ⑥ sprint_start）
--   challenge_end: ref_id = notification_id
--   sprint_*     : ref_id = sprint_id
CREATE TABLE IF NOT EXISTS notification_deliveries (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  kind    TEXT    NOT NULL,   -- 'challenge_end' | 'sprint_reminder' | 'sprint_end' | 'sprint_start'
  ref_id  INTEGER NOT NULL,   -- challenge_end=notification_id, sprint_*=sprint_id
  sent_at TEXT    NOT NULL DEFAULT (datetime('now')),
  UNIQUE(kind, ref_id)
);
