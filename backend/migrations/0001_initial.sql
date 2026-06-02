-- ============================================================
-- BeGit D1 初期スキーマ
--
-- テーブル一覧:
--   users, groups, group_members, fcm_tokens
--   sprints, notifications
--   posts, photos, reactions, comments
-- ============================================================

-- --------------------------------------------------------
-- users
-- iOS: GitHubUser / RepositoryMember
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
  id                      INTEGER PRIMARY KEY AUTOINCREMENT,
  github_id               INTEGER NOT NULL UNIQUE,
  github_login            TEXT    NOT NULL UNIQUE,
  github_name             TEXT,
  avatar_url              TEXT,
  encrypted_access_token  TEXT    NOT NULL,
  created_at              TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- --------------------------------------------------------
-- groups
-- iOS: Repository
--   repo_full_name = Repository.name ("owner/repo")
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS groups (
  id                    INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_full_name        TEXT    NOT NULL UNIQUE,
  owner_user_id         INTEGER NOT NULL REFERENCES users(id),
  sprint_duration_days  INTEGER NOT NULL DEFAULT 7,
  created_at            TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- --------------------------------------------------------
-- group_members
-- iOS: Repository.members
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS group_members (
  group_id    INTEGER NOT NULL REFERENCES groups(id),
  user_id     INTEGER NOT NULL REFERENCES users(id),
  role        TEXT    NOT NULL DEFAULT 'member',  -- 'owner' | 'member'
  auto_joined INTEGER NOT NULL DEFAULT 0,
  joined_at   TEXT    NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (group_id, user_id)
);

-- --------------------------------------------------------
-- fcm_tokens
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS fcm_tokens (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id),
  token      TEXT    NOT NULL UNIQUE,
  updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- --------------------------------------------------------
-- sprints
-- グループのスプリント期間を管理する
--
-- グループ作成時に第1スプリント (index_num=0) を自動作成する。
-- ends_at = started_at + sprint_duration_days
-- スプリント終了後は Cron が次スプリントを INSERT する。
--
-- 現在アクティブなスプリント:
--   WHERE group_id = ? AND started_at <= now() AND ends_at > now()
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS sprints (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id   INTEGER NOT NULL REFERENCES groups(id),
  index_num  INTEGER NOT NULL DEFAULT 0,
  started_at TEXT    NOT NULL DEFAULT (datetime('now')),
  ends_at    TEXT    NOT NULL,
  UNIQUE(group_id, index_num)
);

-- --------------------------------------------------------
-- notifications (BeGit Time 通知発行)
-- iOS: RepositoryNotification
--
-- sprint_id で「どのスプリントの通知か」を紐付ける。
-- UNIQUE(sprint_id, sent_by) で 1スプリント1人1回を保証する。
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS notifications (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  sprint_id INTEGER NOT NULL REFERENCES sprints(id),
  sent_by   INTEGER NOT NULL REFERENCES users(id),
  message   TEXT    NOT NULL DEFAULT '今なに作ってる？',
  sent_at   TEXT    NOT NULL DEFAULT (datetime('now')),
  UNIQUE(sprint_id, sent_by)
);

-- --------------------------------------------------------
-- posts (統一投稿テーブル)
-- iOS: RepositoryActivity (Dashboard) + 将来の FeedPost (Feed)
--
-- post_type:
--   'commit'       コミット報告
--   'pull_request' PR オープン / レビュー
--   'issue'        Issue 対応
--   'review'       コードレビュー完了
--   'memo'         進捗メッセージ（今は作業できないが近況を共有）
--
-- status (on_time / late):
--   notification_id != NULL のとき、
--   post.created_at <= notification.sent_at + 1h → 'on_time'
--   post.created_at >  notification.sent_at + 1h → 'late'
--   スプリント終了後も未投稿 → Cron が 'missed' を INSERT
--
-- blur 制御はサーバー側で算出:
--   リクエストユーザーが通知後に未投稿なら
--   body / repo_full_name / latest_commit_message / photos を null で返す
--
-- UNIQUE(notification_id, user_id):
--   1通知に対して1ユーザー1投稿
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS posts (
  id                     INTEGER PRIMARY KEY AUTOINCREMENT,
  notification_id        INTEGER REFERENCES notifications(id),
  user_id                INTEGER NOT NULL REFERENCES users(id),
  group_id               INTEGER NOT NULL REFERENCES groups(id),
  post_type              TEXT    NOT NULL DEFAULT 'commit',
  body                   TEXT,
  repo_full_name         TEXT,
  branch_name            TEXT,
  commit_count           INTEGER NOT NULL DEFAULT 0,
  additions              INTEGER NOT NULL DEFAULT 0,
  deletions              INTEGER NOT NULL DEFAULT 0,
  latest_commit_message  TEXT,
  tags                   TEXT,              -- JSON 配列 ex. '["Go","Swift"]'
  privacy_level          TEXT    NOT NULL DEFAULT 'group',
  status                 TEXT,              -- 'on_time' | 'late' | 'missed' | NULL
  created_at             TEXT    NOT NULL DEFAULT (datetime('now')),
  UNIQUE(notification_id, user_id)
);

-- --------------------------------------------------------
-- photos
-- R2 バケット "begit-photos" のオブジェクトキーを保持
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS photos (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  post_id    INTEGER NOT NULL REFERENCES posts(id),
  r2_key     TEXT    NOT NULL,
  photo_type TEXT    NOT NULL DEFAULT 'code_screenshot',
  -- 'code_screenshot' | 'desk' | 'environment'
  created_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- --------------------------------------------------------
-- reactions
-- Dashboard: 'heart' | 'check'
-- Feed:      'lgtm' | 'watching' | 'grass' | 'strong' | 'review' | 'merge'
-- iOS 側が post_type に応じて絵文字セットを切り替える
-- UNIQUE(post_id, user_id, reaction_type): 同じ絵文字は1回のみ
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS reactions (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  post_id       INTEGER NOT NULL REFERENCES posts(id),
  user_id       INTEGER NOT NULL REFERENCES users(id),
  reaction_type TEXT    NOT NULL,
  UNIQUE(post_id, user_id, reaction_type)
);

-- --------------------------------------------------------
-- comments
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS comments (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  post_id    INTEGER NOT NULL REFERENCES posts(id),
  user_id    INTEGER NOT NULL REFERENCES users(id),
  body       TEXT    NOT NULL,
  created_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- --------------------------------------------------------
-- github_webhook_deliveries (Webhook 冪等性)
-- GitHub は Webhook の配信失敗時に同じ内容をリトライする。
-- X-GitHub-Delivery ヘッダーの UUID を記録し、
-- 処理済みの配信を受け取ったら即 200 を返して無視する。
--
-- Go ハンドラーの処理順:
--   1. X-Hub-Signature-256 を HMAC 検証
--   2. delivery_id を INSERT — 重複なら UNIQUE エラー → 200 で返す
--   3. イベント処理（posts INSERT / FCM 送信）
-- --------------------------------------------------------
CREATE TABLE IF NOT EXISTS github_webhook_deliveries (
  delivery_id TEXT    PRIMARY KEY,   -- X-GitHub-Delivery ヘッダーの値（UUID）
  event_type  TEXT    NOT NULL,      -- X-GitHub-Event ヘッダーの値 ('push', 'pull_request_review')
  received_at TEXT    NOT NULL DEFAULT (datetime('now'))
);
