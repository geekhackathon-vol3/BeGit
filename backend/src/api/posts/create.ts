import { errorResponse } from "../errors";
import { decodeGoObject } from "../gojson";
import type { RouteContext } from "../types";
import { formatRFC3339, parseGoTimestamp, POST_COLUMNS, type PostRow, toPostJSON } from "./post";

// 投稿作成 POST /groups/:id/posts のうち、Worker が処理するのは写真の下書き（content_source = photo）だけ。
// GitHub / 手入力の投稿は GitHub API を呼ぶため、これまでどおりコンテナ（Go）が処理する（#132）。

export interface PhotoDraftInput {
  body: string | null;
  notificationID: number | null;
  postType: string;
}

// Go の handler.CreatePostRequest の項目と型
const CREATE_POST_FIELDS = {
  body: "*string",
  notification_id: "*int64",
  post_type: "string",
  content_source: "string",
  commit_sha: "*string",
  pull_request_number: "*int64",
  github_login: "string",
  repo_full_name: "string",
} as const;

const POST_TYPES = new Set(["commit", "pull_request", "memo"]);

// 本文が大きいものは写真の下書きではないので、読まずに転送する
const MAX_BODY_BYTES = 64 * 1024;

// claimPhotoDraft は、Go が写真の下書きとして処理するリクエストなら、その入力を返す。
// それ以外（GitHub / 手入力、Go がエラーにする本文、Go と読み方がずれうる本文）は undefined を返し、コンテナへ転送させる。
// 転送に元の本文を使えるよう、読むのは request.clone()。認証より前に呼ばれる
export async function claimPhotoDraft(request: Request): Promise<PhotoDraftInput | undefined> {
  if (Number(request.headers.get("Content-Length") ?? "0") > MAX_BODY_BYTES) return undefined;
  const bytes = await readCloneUpTo(request, MAX_BODY_BYTES);
  if (bytes === null) return undefined;
  // 先頭の BOM を残す（Go は BOM 付きの JSON をエラーにするので、JSON.parse でも失敗させて転送する）
  const text = new TextDecoder("utf-8", { fatal: false, ignoreBOM: true }).decode(bytes);
  const fields = decodeGoObject(text, CREATE_POST_FIELDS);
  if (!fields || fields.content_source !== "photo") return undefined;

  // Go の CreatePost と同じく、post_type の既定は commit。不正な post_type は Go のエラー文言に任せる
  const postType = (fields.post_type as string | undefined) || "commit";
  if (!POST_TYPES.has(postType)) return undefined;
  return {
    body: (fields.body as string | null | undefined) ?? null,
    notificationID: (fields.notification_id as number | null | undefined) ?? null,
    postType,
  };
}

interface NotificationRow {
  sent_at: unknown;
  stopped_at: unknown;
  ended_at: unknown;
  sprint_id: number | null;
  sprint_group_id: number | null;
  ends_at: unknown;
}

const PHOTO_REPORT_REQUIRED = "validation error: photo report requires a BeGit Time and memo post type";

// createPhotoDraft は Go の postService.CreatePost の写真の経路（content_source = photo）と同じルールで下書きを作る。
//   完了したプロジェクト → 409 / 通知がない・別グループ → 400 / 止めた・スプリントが終わった → 409
//   同じ BeGit Time! の自分の投稿があればそれを返し、missed なら 409
// Go は確認ごとに D1 へ往復するが、読む3つを batch() で1回にまとめ、作るときは INSERT と読み直しで1回にする。
export async function createPhotoDraft({ db, caller, input }: RouteContext): Promise<Response> {
  const { body, notificationID, postType } = input as PhotoDraftInput;
  if (notificationID === null) return errorResponse(400, PHOTO_REPORT_REQUIRED);

  const selectOwnPost = db.prepare(`SELECT ${POST_COLUMNS} FROM posts WHERE user_id = ? AND notification_id = ? LIMIT 1`)
    .bind(caller.userID, notificationID);
  const [groups, notifications, ownPosts] = await db.batch<unknown>([
    db.prepare("SELECT completed_at FROM groups WHERE id = ? LIMIT 1").bind(caller.groupID),
    db.prepare(
      `SELECT n.sent_at, n.stopped_at, n.ended_at, s.id AS sprint_id, s.group_id AS sprint_group_id, s.ends_at
       FROM notifications n LEFT JOIN sprints s ON s.id = n.sprint_id WHERE n.id = ? LIMIT 1`,
    ).bind(notificationID),
    selectOwnPost,
  ]);

  const group = groups.results[0] as { completed_at: unknown } | undefined;
  // router でメンバーと確かめたグループなので、ないのは Go と同じく 500
  if (!group) throw new Error(`group ${caller.groupID} not found`);
  if (isSet(group.completed_at) && parseGoTimestamp(group.completed_at)) return errorResponse(409, "conflict");

  const notification = notifications.results[0] as NotificationRow | undefined;
  if (!notification) return errorResponse(400, "validation error: notification not found");
  // Go は sent_at / ended_at を読めないと 500 にする
  if (typeof notification.sent_at === "string" && !parseGoTimestamp(notification.sent_at)) {
    throw new Error(`notification ${notificationID} has invalid sent_at`);
  }
  if (isSet(notification.ended_at) && !parseGoTimestamp(notification.ended_at)) {
    throw new Error(`notification ${notificationID} has invalid ended_at`);
  }
  if (notification.sprint_id === null) throw new Error(`sprint of notification ${notificationID} not found`);
  if (notification.sprint_group_id !== caller.groupID) {
    return errorResponse(400, "validation error: notification does not belong to group");
  }
  // 写真の報告はスプリントの終わりまで受け付ける。Go は読めない ends_at をゼロ時刻（締め切り済み）として扱う
  const deadline = parseGoTimestamp(notification.ends_at)?.getTime() ?? -Infinity;
  const stopped = isSet(notification.stopped_at) && parseGoTimestamp(notification.stopped_at) !== null;
  if (stopped || !(Date.now() < deadline)) {
    return errorResponse(409, "conflict: notification is no longer accepting posts");
  }
  if (postType !== "memo") return errorResponse(400, PHOTO_REPORT_REQUIRED);

  // GitHub の報告がすでにあれば、写真はそこに付ける（Go と同じくそれを返す）
  const existing = ownPosts.results[0] as PostRow | undefined;
  if (existing) {
    if (existing.status === "missed") return errorResponse(409, "conflict");
    return Response.json(toPostJSON(existing), { status: 201 });
  }

  // 同時に作られたら、UNIQUE(notification_id, user_id) に当たった方は先にできた行を返す（Go と同じ）
  const [, created] = await db.batch<PostRow>([
    db.prepare(
      `INSERT INTO posts (notification_id, user_id, group_id, post_type, body, repo_full_name, branch_name, commit_count, additions, deletions, latest_commit_message, status, is_draft, created_at)
       VALUES (?, ?, ?, 'memo', ?, NULL, NULL, 0, 0, 0, NULL, NULL, 1, ?)
       ON CONFLICT (notification_id, user_id) DO NOTHING`,
    ).bind(notificationID, caller.userID, caller.groupID, body, formatRFC3339(new Date())),
    selectOwnPost,
  ]);
  const post = created.results[0];
  if (!post) throw new Error(`draft for notification ${notificationID} disappeared while creating`);
  return Response.json(toPostJSON(post), { status: 201 });
}

// Go の scan は、空でない文字列のときだけ時刻として読む
function isSet(value: unknown): value is string {
  return typeof value === "string" && value !== "";
}

// readCloneUpTo は request.clone() の本文を max バイトまで読む。超えたら null を返す。
// clone は元の本文と tee() でつながっていて、片方の取り消しはもう片方も取り消されるまで終わらないので、取り消しは待たない
// （body.ts の readBodyUpTo は取り消しを待つため使わない）
async function readCloneUpTo(request: Request, max: number): Promise<Uint8Array | null> {
  const clone = request.clone();
  if (!clone.body) return new Uint8Array();
  const reader = clone.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > max) {
      reader.cancel().catch(() => {});
      return null;
    }
    chunks.push(value);
  }
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return bytes;
}
