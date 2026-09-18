-- ============================================================
-- notifications に ended_at を追加する（発行者による途中中断）
--
-- BeGit Time! の発行者がチャレンジを途中で終了できるようにする。
-- 中断は「締め切りを今にする」扱い: ended_at 以降は進行中とみなさず（即再発行可）、
-- On Time / Late の判定と ③ challenge_end の送信対象抽出も ended_at を締め切りとして使う。
-- NULL = 中断なし（従来どおり sent_at + 1h が締め切り）。
-- ============================================================
ALTER TABLE notifications ADD COLUMN ended_at TEXT;
