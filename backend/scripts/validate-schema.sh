#!/usr/bin/env bash
# Validate migrations against SQLite (D1-compatible dialect)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MIGRATION="$ROOT/migrations/0001_initial.sql"
DB="$(mktemp /tmp/begit-schema-XXXXXX.db)"

cleanup() {
  rm -f "$DB"
}
trap cleanup EXIT

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "error: sqlite3 is required" >&2
  exit 1
fi

echo "Applying $MIGRATION to temp database..."
sqlite3 "$DB" < "$MIGRATION"

echo "Checking tables..."
EXPECTED="be_time_notifications
comments
fcm_tokens
github_webhook_deliveries
group_members
groups
photos
post_tags
posts
reactions
sprints
tags
users"

ACTUAL=$(sqlite3 "$DB" "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;")
if [ "$(echo "$ACTUAL" | sort)" != "$(echo "$EXPECTED" | sort)" ]; then
  echo "error: table mismatch" >&2
  echo "expected:" >&2
  echo "$EXPECTED" >&2
  echo "actual:" >&2
  echo "$ACTUAL" >&2
  exit 1
fi

echo "Seeding constraint test data..."
sqlite3 "$DB" <<'SQL'
PRAGMA foreign_keys = ON;
INSERT INTO users (id, github_id, github_login, username, access_token_encrypted)
VALUES ('u1', 1, 'alice', 'Alice', 'enc');
INSERT INTO "groups" (id, name, repo_full_name, sprint_duration_days, created_by)
VALUES ('g1', 'Team', 'org/repo', 7, 'u1');
INSERT INTO group_members (group_id, user_id, role) VALUES ('g1', 'u1', 'owner');
INSERT INTO sprints (id, group_id, index_num, started_at, ends_at)
VALUES ('s1', 'g1', 1, '2026-01-01T00:00:00Z', '2026-01-08T00:00:00Z');
INSERT INTO be_time_notifications (id, group_id, sprint_id, sent_by, message)
VALUES ('n1', 'g1', 's1', 'u1', 'BeGit Time');
SQL

echo "Checking UNIQUE(sprint_id, sent_by)..."
set +e
DUPE=$(sqlite3 "$DB" "INSERT INTO be_time_notifications (id, group_id, sprint_id, sent_by, message) VALUES ('n2', 'g1', 's1', 'u1', 'duplicate');" 2>&1)
DUPE_RC=$?
set -e

if [ "$DUPE_RC" -eq 0 ]; then
  echo "error: expected UNIQUE(sprint_id, sent_by) violation" >&2
  exit 1
fi

echo "Checking sprint date CHECK..."
set +e
BAD_SPRINT=$(sqlite3 "$DB" "INSERT INTO sprints (id, group_id, index_num, started_at, ends_at) VALUES ('s2', 'g1', 2, '2026-01-10T00:00:00Z', '2026-01-01T00:00:00Z');" 2>&1)
BAD_SPRINT_RC=$?
set -e

if [ "$BAD_SPRINT_RC" -eq 0 ]; then
  echo "error: expected CHECK(ends_at > started_at) violation" >&2
  exit 1
fi

echo "OK — schema valid, constraints enforced"
