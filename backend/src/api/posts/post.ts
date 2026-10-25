// posts テーブルの行と、Go の handler.PostJSON と同じ形の応答。

const POST_COLUMN_NAMES = [
  "id", "notification_id", "user_id", "group_id", "post_type", "body", "repo_full_name", "branch_name",
  "commit_count", "additions", "deletions", "latest_commit_message", "status", "is_draft", "created_at",
];

export const POST_COLUMNS = POST_COLUMN_NAMES.join(", ");

// postColumns は JOIN するときの列（例 postColumns("p") → "p.id, p.notification_id, …"）
export function postColumns(alias: string): string {
  return POST_COLUMN_NAMES.map((column) => `${alias}.${column}`).join(", ");
}

// 写真の報告を受け付けているかは SQL で判定する（steering の tech.md「書き込みの前提は SQL で守る」）。
// 読み取りでは結果を列として読んで応答を決め、書き込みでは同じ条件を WHERE に付けて、確かめた後に状態が変わっても書かない。
// 引数は SQL の式をそのまま埋め込むので、"?" や列名などの定数だけを渡す

// groupCompleted は、groups の別名 alias のプロジェクトが完了しているかの条件。
// 空文字は完了していないとみなす（Go の scan は空文字を時刻として読まない）
export function groupCompleted(alias: string): string {
  return `COALESCE(${alias}.completed_at, '') <> ''`;
}

// acceptsPhotoReports は、通知 notificationIDExpr が写真の報告を受け付けているかの条件。
// 止めていない・スプリントの終わり前・プロジェクトが完了していない。ends_at は書き方が混ざりうるので julianday で比べる（読めなければ締め切り済み）
export function acceptsPhotoReports(notificationIDExpr: string): string {
  return `EXISTS (SELECT 1 FROM notifications pn JOIN sprints ps ON ps.id = pn.sprint_id JOIN groups pg ON pg.id = ps.group_id
    WHERE pn.id = ${notificationIDExpr} AND COALESCE(pn.stopped_at, '') = '' AND NOT ${groupCompleted("pg")}
      AND julianday(ps.ends_at) > julianday('now'))`;
}

export const NOT_ACCEPTING_POSTS = "conflict: notification is no longer accepting posts";

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
// Go の time.Parse は、レイアウトになくても秒の後ろの小数（区切りは . か ,）を読む
const RFC3339 = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:[.,]\d+)?(Z|[+-]\d{2}:\d{2})$/;
const SQLITE_DATETIME = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2}):(\d{2})(?:[.,]\d+)?$/;

// formatTimestamp は Go の scanPost → CreatedAt.UTC().Format(time.RFC3339) と同じ文字列にする。
// 読めなければゼロ時刻にする（Go と同じ）。
export function formatTimestamp(value: unknown): string {
  const date = parseGoTimestamp(value);
  return date ? formatRFC3339(date) : ZERO_TIME;
}

// formatRFC3339 は Go の t.UTC().Format(time.RFC3339) と同じく、秒までの UTC にする
export function formatRFC3339(date: Date): string {
  return date.toISOString().replace(/\.\d{3}Z$/, "Z");
}

// parseGoTimestamp は Go の time.Parse(time.RFC3339, v)、だめなら time.Parse("2006-01-02 15:04:05", v) と同じ規則で読む。
// SQLite の datetime('now') の形式は UTC として読む。読めなければ null
export function parseGoTimestamp(value: unknown): Date | null {
  if (typeof value !== "string") return null;
  const rfc = RFC3339.exec(value);
  const sqlite = rfc ? null : SQLITE_DATETIME.exec(value);
  const m = rfc ?? sqlite;
  if (!m) return null;

  const [year, month, day, hour, minute, second] = m.slice(1, 7).map(Number);
  // Date.UTC は 0〜99 年を 1900 年代にし、範囲外の値を繰り上げるので、setUTCFullYear で組み立てて確かめる
  const date = new Date(0);
  date.setUTCFullYear(year, month - 1, day);
  date.setUTCHours(hour, minute, second, 0);
  if (date.getUTCFullYear() !== year || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day ||
      date.getUTCHours() !== hour || date.getUTCMinutes() !== minute || date.getUTCSeconds() !== second) {
    return null;
  }

  const zone = rfc ? m[7] : "Z";
  if (zone !== "Z") {
    const zoneHours = +zone.slice(1, 3);
    const zoneMinutes = +zone.slice(4, 6);
    // Go はタイムゾーンを「時 24 以下、分 60 以下」なら受け付ける
    if (zoneHours > 24 || zoneMinutes > 60) return null;
    const sign = zone[0] === "-" ? -1 : 1;
    date.setTime(date.getTime() - sign * (zoneHours * 60 + zoneMinutes) * 60_000);
  }
  return date;
}
