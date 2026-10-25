import { readBodyUpTo } from "../body";
import { errorResponse } from "../errors";
import { parseID } from "../params";
import type { RouteContext } from "../types";
import { POST_COLUMNS, type PostRow, toPostJSON } from "./post";

// confirmPost は POST /groups/:id/posts/:postId/confirm。
// Go の PostHandler.Confirm → postService.ConfirmPost と同じルールで下書きを確定する。
//   別グループの投稿 → 404 / 本人以外 → 403 / 確定済み → そのまま返す（body は無視）
//   BeGit Time! の写真報告（memo かつ通知つき）→ 409。写真のアップロードでだけ確定する
//   router の認証（owner / poster）を通った後に呼ばれる
export async function confirmPost({ request, db, params, caller }: RouteContext): Promise<Response> {
  const rawPostID = params[1];
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

  // 本文の上書き・確定・取得を1回の往復にまとめる（batch() はトランザクションとして実行される）。
  // 読んだ後に別のリクエストが確定していたら、確定済みの本文を上書きしないよう is_draft = 1 に限る
  const statements = [];
  if (body !== undefined) {
    statements.push(db.prepare("UPDATE posts SET body = ? WHERE id = ? AND is_draft = 1").bind(body, postID.value));
  }
  statements.push(db.prepare("UPDATE posts SET is_draft = 0 WHERE id = ? AND is_draft = 1").bind(postID.value));
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
  const bytes = await readBodyUpTo(request, MAX_BODY_BYTES);
  if (bytes === null) return errorResponse(413, "request body too large");
  // 先頭の BOM を残す（Go は BOM 付きの JSON を読めず 400 にする）。不正な UTF-8 は Go と同じく U+FFFD にする
  const text = new TextDecoder("utf-8", { fatal: false, ignoreBOM: true }).decode(bytes);
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
