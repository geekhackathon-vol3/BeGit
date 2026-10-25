import { readBodyUpTo } from "../body";
import { errorResponse } from "../errors";
import { parseID } from "../params";
import type { NotificationQueueMessage, RouteContext } from "../types";

// Go の service.allowedReactionTypes
const REACTION_TYPES = new Set(["heart", "thumbsup", "celebrate", "fire", "rocket"]);

// 種類だけを送る API なので、これより大きいボディは読まずに 413 にする（Go には上限がない）
const MAX_BODY_BYTES = 64 * 1024;

interface ReactionRow {
  id: number;
  post_id: number;
  user_id: number;
  reaction_type: string;
  login: string | null;
  avatar_url: string | null;
}

// リアクションを付けられる投稿（Go の reactionService.getPostInGroup）。下書き・missed・別グループの投稿は 404
const REACTABLE_POST =
  "SELECT user_id FROM posts WHERE id = ? AND group_id = ? AND is_draft = 0 AND (status IS NULL OR status != 'missed')";

// createReaction は POST /groups/:id/posts/:postId/reactions。
// Go の ReactionHandler.Create → reactionService.AddReaction と同じルールで付け、投稿のリアクション一覧を返す。
//   1人1投稿1つ。前のものを消して入れ直す。種類が不正なら 400（Go は 500 を返していた）
//   自分以外の投稿なら、作者への通知を Queue に積む（送るのは Go の裏方のコンテナ。#134）
// Go は投稿の確認・削除・追加・一覧で D1 へ4往復するが、投稿の確認を条件にして1回の batch() にまとめる。
export async function createReaction({ request, env, db, params, caller }: RouteContext): Promise<Response> {
  const postID = parseID(params[1]);
  if (!postID.valid) return errorResponse(400, "invalid post id");

  const reactionType = await readReactionType(request);
  if (reactionType instanceof Response) return reactionType;
  if (reactionType === "") return errorResponse(400, "reaction_type is required");
  if (!REACTION_TYPES.has(reactionType)) return errorResponse(400, "invalid reaction type");
  if (postID.value === null) return errorResponse(404, "post not found");

  const target = [postID.value, caller.groupID];
  // 投稿がリアクションを付けられないときは、DELETE / INSERT が何もしないよう、同じ条件を付ける
  const [posts, , , reactions] = await db.batch<unknown>([
    db.prepare(REACTABLE_POST).bind(...target),
    db.prepare(`DELETE FROM reactions WHERE post_id = ? AND user_id = ? AND EXISTS (${REACTABLE_POST})`)
      .bind(postID.value, caller.userID, ...target),
    db.prepare(
      `INSERT OR IGNORE INTO reactions (post_id, user_id, reaction_type) SELECT ?, ?, ? WHERE EXISTS (${REACTABLE_POST})`,
    ).bind(postID.value, caller.userID, reactionType, ...target),
    db.prepare(
      `SELECT rx.id, rx.post_id, rx.user_id, rx.reaction_type, u.github_login AS login, u.avatar_url
       FROM reactions rx INNER JOIN users u ON rx.user_id = u.id
       WHERE rx.post_id = ? ORDER BY rx.id ASC`,
    ).bind(postID.value),
  ]);

  const post = posts.results[0] as { user_id: number } | undefined;
  if (!post) return errorResponse(404, "post not found");

  if (post.user_id !== caller.userID) {
    await notifyAuthor(env, { type: "reaction", group_id: caller.groupID, post_id: postID.value, actor_user_id: caller.userID, author_user_id: post.user_id });
  }

  return Response.json({
    reactions: (reactions.results as ReactionRow[]).map((row) => ({
      id: row.id,
      post_id: row.post_id,
      user_id: row.user_id,
      reaction_type: row.reaction_type,
      // Go は NULL を空文字にする
      login: row.login ?? "",
      avatar_url: row.avatar_url ?? "",
    })),
  }, { status: 201 });
}

// notifyAuthor は通知を Queue に積む。Go と同じく、通知の失敗ではリアクションを失敗にしない
async function notifyAuthor(env: RouteContext["env"], message: ReactionNotification): Promise<void> {
  if (!env.NOTIFICATION_QUEUE) {
    console.warn("[reactions.create] NOTIFICATION_QUEUE is not bound; notification skipped");
    return;
  }
  try {
    await env.NOTIFICATION_QUEUE.send(message);
  } catch (error) {
    console.error(`[reactions.create] queue send failed post_id=${message.post_id}`, error);
  }
}

type ReactionNotification = Extract<NotificationQueueMessage, { type: "reaction" }>;

// readReactionType は Go の ShouldBindJSON と同じく {"reaction_type": string} を読む。
// 読めない本文は 400、キーがない・null は空文字（Go と同じく「必須」のエラーにする）。
// Go と違い、JSON の後ろに続くゴミ（{"reaction_type":"heart"}abc など）は 400 にする（posts.confirm と同じ）。
async function readReactionType(request: Request): Promise<string | Response> {
  if (Number(request.headers.get("Content-Length") ?? "0") > MAX_BODY_BYTES) {
    return errorResponse(413, "request body too large");
  }
  const bytes = await readBodyUpTo(request, MAX_BODY_BYTES);
  if (bytes === null) return errorResponse(413, "request body too large");
  // 先頭の BOM を残す（Go は BOM 付きの JSON を読めず 400 にする）
  const text = new TextDecoder("utf-8", { fatal: false, ignoreBOM: true }).decode(bytes);

  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return errorResponse(400, "invalid request body");
  }
  if (parsed === null) return "";
  if (typeof parsed !== "object" || Array.isArray(parsed)) return errorResponse(400, "invalid request body");
  // Go の encoding/json と同じく、キーは大文字小文字を区別せず、後に出たものを使う。null は読み飛ばし、文字列でない値があればエラー
  let value = "";
  for (const [key, v] of Object.entries(parsed)) {
    if (key.toLowerCase() !== "reaction_type" || v === null) continue;
    if (typeof v !== "string") return errorResponse(400, "invalid request body");
    value = v;
  }
  return value;
}
