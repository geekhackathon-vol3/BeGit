import { authorizeGroup, GROUP_ROLE_OWNER, GROUP_ROLE_POSTER } from "../auth";
import { errorResponse } from "../errors";
import { parseID } from "../params";
import type { RouteContext } from "../types";
import { POST_COLUMNS, type PostRow, toPostJSON } from "./post";

// confirmPost は POST /groups/:id/posts/:postId/confirm。
// Go の PostHandler.Confirm → postService.ConfirmPost と同じルールで下書きを確定する。
//   別グループの投稿 → 404 / 本人以外 → 403 / 確定済み → そのまま返す（body は無視）
//   BeGit Time! の写真報告（memo かつ通知つき）→ 409。写真のアップロードでだけ確定する
export async function confirmPost({ request, env, db, params }: RouteContext): Promise<Response> {
  const [rawGroupID, rawPostID] = params;
  const caller = await authorizeGroup(request, env, db, rawGroupID, {
    kind: "roles",
    roles: [GROUP_ROLE_OWNER, GROUP_ROLE_POSTER],
  });
  if (caller instanceof Response) return caller;

  const postID = parseID(rawPostID);
  if (!postID.valid) return errorResponse(400, "invalid post id");

  const body = await readConfirmBody(request);
  if (body instanceof Response) return body;

  if (postID.value === null) return errorResponse(404, "not found");
  const selectPost = db.prepare(`SELECT ${POST_COLUMNS} FROM posts WHERE id = ? LIMIT 1`).bind(postID.value);

  const post = await db.first<PostRow>(selectPost);
  if (!post || post.group_id !== caller.groupID) return errorResponse(404, "not found");
  if (post.user_id !== caller.userID) return errorResponse(403, "forbidden");

  if (!post.is_draft) return Response.json(toPostJSON(post));
  if (post.post_type === "memo" && post.notification_id !== null) {
    return errorResponse(409, "photo upload required");
  }

  // 本文の上書き・確定・取得を1回の往復にまとめる（batch() はトランザクションとして実行される）
  const statements = [];
  if (body !== undefined) {
    statements.push(db.prepare("UPDATE posts SET body = ? WHERE id = ?").bind(body, postID.value));
  }
  statements.push(db.prepare("UPDATE posts SET is_draft = 0 WHERE id = ?").bind(postID.value));
  statements.push(selectPost);
  const results = await db.batch<PostRow>(statements);
  const confirmed = results[results.length - 1].results[0];
  if (!confirmed) throw new Error(`post ${postID.value} disappeared while confirming`);
  return Response.json(toPostJSON(confirmed));
}

// 本文だけを送る API なので、これより大きいボディは読まずに 413 にする（Go には上限がない）
const MAX_BODY_BYTES = 64 * 1024;

// readConfirmBody は Go の ShouldBindJSON と同じく、任意の {"body": string|null} を読む。
// 空のボディは確定のみ。読めない JSON は、Content-Length が正のときだけ 400 にする（Go と同じ）。
// Go と違い、JSON の後ろに続くゴミ（{"body":"x"}abc など）は 400 にする。
async function readConfirmBody(request: Request): Promise<string | undefined | Response> {
  const contentLength = Number(request.headers.get("Content-Length") ?? "0");
  if (contentLength > MAX_BODY_BYTES) return errorResponse(413, "request body too large");
  const bytes = await request.arrayBuffer();
  if (bytes.byteLength > MAX_BODY_BYTES) return errorResponse(413, "request body too large");
  const text = new TextDecoder().decode(bytes);
  const invalid = () => (contentLength > 0 ? errorResponse(400, "invalid request body") : undefined);
  if (text.trim() === "") return invalid();

  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return invalid();
  }
  if (parsed === null) return undefined;
  if (typeof parsed !== "object" || Array.isArray(parsed)) return invalid();
  // Go の encoding/json と同じく、キーは大文字小文字を区別せず、後に出たものを使う
  let value: unknown;
  for (const [key, v] of Object.entries(parsed)) {
    if (key.toLowerCase() === "body") value = v;
  }
  if (value === undefined || value === null) return undefined;
  if (typeof value !== "string") return invalid();
  return value;
}
