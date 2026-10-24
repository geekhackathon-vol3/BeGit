// posts テーブルの行と、Go の handler.PostJSON と同じ形の応答。

export const POST_COLUMNS =
  "id, notification_id, user_id, group_id, post_type, body, repo_full_name, branch_name, commit_count, additions, deletions, latest_commit_message, status, is_draft, created_at";

export interface PostRow {
  id: number;
  notification_id: number | null;
  user_id: number;
  group_id: number;
  post_type: string;
  body: string | null;
  repo_full_name: string | null;
  branch_name: string | null;
  commit_count: number | null;
  additions: number | null;
  deletions: number | null;
  latest_commit_message: string | null;
  status: string | null;
  is_draft: number;
  created_at: string;
}

// PostJSON は Go の handler.PostJSON。null の項目も省かずに返す。
export interface PostJSON {
  id: number;
  notification_id: number | null;
  user_id: number;
  post_type: string;
  body: string | null;
  repo_full_name: string | null;
  commit_count: number;
  additions: number;
  deletions: number;
  latest_commit_message: string | null;
  status: string | null;
  created_at: string;
}

export function toPostJSON(row: PostRow): PostJSON {
  return {
    id: row.id,
    notification_id: row.notification_id ?? null,
    user_id: row.user_id,
    post_type: row.post_type,
    body: row.body ?? null,
    repo_full_name: row.repo_full_name ?? null,
    commit_count: toInt(row.commit_count),
    additions: toInt(row.additions),
    deletions: toInt(row.deletions),
    latest_commit_message: row.latest_commit_message ?? null,
    status: row.status ?? null,
    created_at: formatTimestamp(row.created_at),
  };
}

function toInt(value: number | null): number {
  return typeof value === "number" ? Math.trunc(value) : 0;
}

const ZERO_TIME = "0001-01-01T00:00:00Z";
const RFC3339 = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/;
const SQLITE_DATETIME = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2}):(\d{2})$/;

// formatTimestamp は Go の scanPost → CreatedAt.UTC().Format(time.RFC3339) と同じ文字列にする。
// RFC3339 か SQLite の datetime('now') の形式を読み、読めなければゼロ時刻にする（Go と同じ）。
export function formatTimestamp(value: unknown): string {
  if (typeof value !== "string") return ZERO_TIME;
  const rfc = RFC3339.exec(value);
  const sqlite = rfc ? null : SQLITE_DATETIME.exec(value);
  const m = rfc ?? sqlite;
  if (!m) return ZERO_TIME;

  const [year, month, day, hour, minute, second] = m.slice(1, 7).map(Number);
  // Date.UTC は 0〜99 年を 1900 年代にし、範囲外の値を繰り上げるので、setUTCFullYear で組み立てて確かめる
  const date = new Date(0);
  date.setUTCFullYear(year, month - 1, day);
  date.setUTCHours(hour, minute, second, 0);
  if (date.getUTCFullYear() !== year || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day ||
      date.getUTCHours() !== hour || date.getUTCMinutes() !== minute || date.getUTCSeconds() !== second) {
    return ZERO_TIME;
  }

  const zone = rfc ? m[7] : "Z";
  if (zone !== "Z") {
    const zoneHours = +zone.slice(1, 3);
    const zoneMinutes = +zone.slice(4, 6);
    if (zoneHours > 23 || zoneMinutes > 59) return ZERO_TIME;
    const sign = zone[0] === "-" ? -1 : 1;
    date.setTime(date.getTime() - sign * (zoneHours * 60 + zoneMinutes) * 60_000);
  }
  return date.toISOString().replace(/\.\d{3}Z$/, "Z");
}
