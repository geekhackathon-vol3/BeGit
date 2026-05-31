-- BeGit initial schema for Cloudflare D1 (SQLite)
-- Migration: 0001_initial.sql
--
-- Review notes (2026-05-31):
--   - "groups" is a SQL reserved word; table is quoted as "groups"
--   - FK columns indexed per database.md standards
--   - sprints.ends_at must be after started_at
--   - Missed posts require notification_id (batch links per notification)
--
-- Rollback (manual, reverse dependency order):
--   DROP TABLE IF EXISTS github_webhook_deliveries;
--   DROP TABLE IF EXISTS comments;
--   DROP TABLE IF EXISTS reactions;
--   DROP TABLE IF EXISTS photos;
--   DROP TABLE IF EXISTS post_tags;
--   DROP TABLE IF EXISTS tags;
--   DROP TABLE IF EXISTS posts;
--   DROP TABLE IF EXISTS be_time_notifications;
--   DROP TABLE IF EXISTS sprints;
--   DROP TABLE IF EXISTS group_members;
--   DROP TABLE IF EXISTS "groups";
--   DROP TABLE IF EXISTS fcm_tokens;
--   DROP TABLE IF EXISTS users;

PRAGMA foreign_keys = ON;

-- users
CREATE TABLE users (
  id                     TEXT PRIMARY KEY,
  github_id              INTEGER NOT NULL UNIQUE,
  github_login           TEXT NOT NULL UNIQUE,
  username               TEXT NOT NULL,
  avatar_url             TEXT,
  access_token_encrypted TEXT NOT NULL,
  token_expires_at       TEXT,
  created_at             TEXT NOT NULL DEFAULT (datetime('now'))
);

-- fcm_tokens (1 user : N devices)
CREATE TABLE fcm_tokens (
  id                 TEXT PRIMARY KEY,
  user_id            TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  registration_token TEXT NOT NULL UNIQUE,
  platform           TEXT NOT NULL DEFAULT 'ios' CHECK (platform IN ('ios')),
  updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_fcm_tokens_user_id ON fcm_tokens(user_id);

-- groups (MVP: 1 repo per group; quoted — GROUP is SQL reserved)
CREATE TABLE "groups" (
  id                   TEXT PRIMARY KEY,
  name                 TEXT NOT NULL,
  repo_full_name       TEXT NOT NULL,
  sprint_duration_days INTEGER NOT NULL CHECK (sprint_duration_days > 0),
  created_by           TEXT NOT NULL REFERENCES users(id),
  created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_groups_created_by ON "groups"(created_by);
CREATE INDEX idx_groups_repo_full_name ON "groups"(repo_full_name);

-- group_members
CREATE TABLE group_members (
  group_id    TEXT NOT NULL REFERENCES "groups"(id) ON DELETE CASCADE,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role        TEXT NOT NULL CHECK (role IN ('owner', 'member')),
  auto_joined INTEGER NOT NULL DEFAULT 0 CHECK (auto_joined IN (0, 1)),
  joined_at   TEXT NOT NULL DEFAULT (datetime('now')),
  left_at     TEXT,
  PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_group_members_user_id ON group_members(user_id);
CREATE INDEX idx_group_members_group_active ON group_members(group_id, left_at);

-- sprints
CREATE TABLE sprints (
  id         TEXT PRIMARY KEY,
  group_id   TEXT NOT NULL REFERENCES "groups"(id) ON DELETE CASCADE,
  index_num  INTEGER NOT NULL CHECK (index_num >= 1),
  started_at TEXT NOT NULL,
  ends_at    TEXT NOT NULL,
  CHECK (ends_at > started_at),
  UNIQUE (group_id, index_num)
);
CREATE INDEX idx_sprints_group_active ON sprints(group_id, ends_at);

-- be_time_notifications (BeGit Time — not FCM push)
CREATE TABLE be_time_notifications (
  id        TEXT PRIMARY KEY,
  group_id  TEXT NOT NULL REFERENCES "groups"(id) ON DELETE CASCADE,
  sprint_id TEXT NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
  sent_by   TEXT NOT NULL REFERENCES users(id),
  message   TEXT NOT NULL,
  sent_at   TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE (sprint_id, sent_by)
);
CREATE INDEX idx_be_time_notifications_group ON be_time_notifications(group_id, sent_at DESC);
CREATE INDEX idx_be_time_notifications_sprint_id ON be_time_notifications(sprint_id);

-- posts
CREATE TABLE posts (
  id              TEXT PRIMARY KEY,
  notification_id TEXT NOT NULL REFERENCES be_time_notifications(id) ON DELETE CASCADE,
  user_id         TEXT NOT NULL REFERENCES users(id),
  group_id        TEXT NOT NULL REFERENCES "groups"(id),
  repo_name       TEXT,
  branch_name     TEXT,
  commit_count    INTEGER NOT NULL DEFAULT 0 CHECK (commit_count >= 0),
  diff_add        INTEGER NOT NULL DEFAULT 0 CHECK (diff_add >= 0),
  diff_remove     INTEGER NOT NULL DEFAULT 0 CHECK (diff_remove >= 0),
  commit_message  TEXT,
  memo            TEXT,
  privacy_level   TEXT NOT NULL DEFAULT 'group'
                  CHECK (privacy_level IN ('public', 'group', 'private')),
  status          TEXT NOT NULL
                  CHECK (status IN ('on_time', 'late', 'missed')),
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE (notification_id, user_id)
);
CREATE INDEX idx_posts_group_feed ON posts(group_id, created_at DESC);
CREATE INDEX idx_posts_user ON posts(user_id);
CREATE INDEX idx_posts_notification_id ON posts(notification_id);

-- tags + post_tags
CREATE TABLE tags (
  id   TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE post_tags (
  post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  tag_id  TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (post_id, tag_id)
);
CREATE INDEX idx_post_tags_tag_id ON post_tags(tag_id);

-- photos
CREATE TABLE photos (
  id      TEXT PRIMARY KEY,
  post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  r2_key  TEXT NOT NULL,
  type    TEXT NOT NULL CHECK (type IN ('code_screenshot', 'desk', 'environment')),
  blur    INTEGER NOT NULL DEFAULT 0 CHECK (blur IN (0, 1))
);
CREATE INDEX idx_photos_post_id ON photos(post_id);

-- reactions
CREATE TABLE reactions (
  id      TEXT PRIMARY KEY,
  post_id TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type    TEXT NOT NULL CHECK (type IN ('lgtm', 'watching', 'grass', 'strong', 'review', 'merge')),
  UNIQUE (post_id, user_id, type)
);
CREATE INDEX idx_reactions_post_id ON reactions(post_id);

-- comments
CREATE TABLE comments (
  id         TEXT PRIMARY KEY,
  post_id    TEXT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body       TEXT NOT NULL CHECK (length(trim(body)) > 0),
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_comments_post_id ON comments(post_id, created_at);

-- github webhook deduplication (no FK — standalone)
CREATE TABLE github_webhook_deliveries (
  delivery_id TEXT PRIMARY KEY,
  event_type  TEXT NOT NULL,
  received_at TEXT NOT NULL DEFAULT (datetime('now'))
);
