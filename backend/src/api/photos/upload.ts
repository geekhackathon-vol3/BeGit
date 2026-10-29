import { limitBytes } from "../body";
import { errorResponse } from "../errors";
import { parseID } from "../params";
import { acceptsPhotoReports, formatRFC3339, NOT_ACCEPTING_POSTS, parseGoTimestamp, POST_COLUMNS, type PostRow } from "../posts/post";
import { photoStorage, type PhotoStorage } from "../r2";
import type { RouteContext } from "../types";
import { type PhotoRow, toPhotoJSONs } from "./photo";

// Go の service.MaxPhotoBytes と handler.maxUploadBytes
const MAX_PHOTO_BYTES = 10 * 1024 * 1024;
const MAX_UPLOAD_BYTES = 2 * MAX_PHOTO_BYTES + 1024 * 1024;
// Go の challengeWindow。BeGit Time! の通知から1時間以内なら on_time
const CHALLENGE_WINDOW_MS = 60 * 60 * 1000;
// Go の allowedPhotoContentTypes（R2 のキーの拡張子）
const EXTENSIONS: Record<string, string> = { "image/jpeg": "jpg", "image/png": "png", "image/heic": "heic" };

// uploadPhotos は POST /groups/:id/posts/:postId/photos。
// Go の PhotoHandler.Upload → photoService.UploadPhotos と同じ順で確かめ、同じエラーを返す。
//   multipart が読めない・21MB を超える → 413 / main がない → 400
//   別グループ・missed の投稿 → 404 / 本人以外 → 403 / 通知のない下書き → 422
//   写真が保存済み（前回の応答が届かなかった再送）→ 保存済みの写真を返す。下書きなら確定する
//   1枚 10MB まで、形式はバイト列から判定する（jpeg / png。heic はパートの Content-Type と ftyp で判定）→ 違えば 422
//   下書きは確定し、締め切りまでなら on_time、過ぎていれば late にする。
//   通知を止めた・スプリントが終わった・プロジェクトが完了した後は、下書きを確定しない → 409（投稿作成と同じ条件）
//   router の認証（owner / poster）を通った後に呼ばれる
export async function uploadPhotos({ request, env, db, params, caller }: RouteContext): Promise<Response> {
  const postID = parseID(params[1]);
  if (!postID.valid) return errorResponse(400, "invalid post id");

  const form = await readMultipartForm(request);
  if (!form) return errorResponse(413, "request too large or invalid multipart form");
  const main = await readFormFile(form, "main");
  if (!main) return errorResponse(400, "main photo is required");
  const front = await readFormFile(form, "front");

  if (postID.value === null) return errorResponse(404, "post not found");
  const [postResult, photosResult, notificationResult] = await db.batch([
    db.prepare(`SELECT ${POST_COLUMNS} FROM posts WHERE id = ? LIMIT 1`).bind(postID.value),
    db.prepare("SELECT id, post_id, r2_key, photo_type FROM photos WHERE post_id = ? ORDER BY id ASC").bind(postID.value),
    db.prepare(
      `SELECT n.sent_at, n.ended_at, ${acceptsPhotoReports("n.id")} AS accepting
       FROM notifications n WHERE n.id = (SELECT notification_id FROM posts WHERE id = ?)`,
    ).bind(postID.value),
  ]);
  const post = postResult.results[0] as unknown as PostRow | undefined;
  if (!post || post.group_id !== caller.groupID) return errorResponse(404, "post not found");
  if (post.user_id !== caller.userID) return errorResponse(403, "forbidden");
  if (post.status === "missed") return errorResponse(404, "post not found");
  if (post.is_draft && post.notification_id === null) return errorResponse(422, "invalid photo");

  const storage = photoStorage(env);
  const notification = notificationResult.results[0] as unknown as NotificationRow | undefined;
  // R2 に保存する前に断る（確定の UPDATE でも同じ条件をもう一度確かめる）
  if (post.is_draft && notification && !notification.accepting) return errorResponse(409, NOT_ACCEPTING_POSTS);
  // 確定の SQL は R2 に保存する前に組み立てる（通知が読めないときに、保存してから消す手間を省く）。
  // Go は保存の後に時刻を取るので、on_time / late の境目が保存にかかる時間だけずれる
  const confirmation = post.is_draft ? confirmStatement(db, post, notification) : null;

  // 前回のアップロードが成功していて、応答だけが届かなかった場合の再送
  const existing = photosResult.results as unknown as PhotoRow[];
  if (existing.length > 0) {
    if (confirmation) {
      const [, confirmed] = await db.batch([confirmation, selectIsDraft(db, post.id)]);
      if (isStillDraft(confirmed)) return errorResponse(409, NOT_ACCEPTING_POSTS);
    }
    return photosResponse(storage, existing);
  }

  const items: { photoType: string; file: FormFile }[] = [{ photoType: "main", file: main }];
  if (front) items.push({ photoType: "front", file: front });
  const validated: { photoType: string; contentType: string; key: string; file: File }[] = [];
  for (const { photoType, file } of items) {
    if (file.file.size === 0 || file.file.size > MAX_PHOTO_BYTES) return errorResponse(422, "invalid photo");
    const contentType = contentTypeOf(file);
    const extension = EXTENSIONS[contentType];
    if (!extension) return errorResponse(422, "invalid photo");
    validated.push({ photoType, contentType, key: `posts/${postID.value}/${photoType}.${extension}`, file: file.file });
  }

  // R2 に保存してから行を作る。途中で失敗したら、どの行も指していない写真だけを消す（行は batch() なので残らない）。
  // 同じ投稿に同時に送られても、キーは同じ（posts/<id>/<種類>.<拡張子>）なので、別のリクエストの写真を消さないよう行で確かめる
  const uploadedKeys: string[] = [];
  // known がなければ行を読んで確かめる。確かめられないときは、行が指しているかもしれないので消さない
  const removeUnreferenced = async (known?: { r2_key: string }[]) => {
    const rows = known ?? await db.all<{ r2_key: string }>(db.prepare("SELECT r2_key FROM photos WHERE post_id = ?").bind(postID.value))
      .catch(() => null);
    if (!rows) {
      console.error(`[photos.upload] could not check photos of post ${postID.value}; kept uploaded photos`, uploadedKeys);
      return;
    }
    const referenced = new Set(rows.map((row) => row.r2_key));
    await Promise.all(uploadedKeys.filter((key) => !referenced.has(key)).map((key) =>
      storage.delete(key).catch((error) => console.error(`[photos.upload] failed to remove ${key}`, error))
    ));
  };
  for (const item of validated) {
    // 書けたのに失敗が返ることもあるので、put の前に片付けの対象にする（delete は何度呼んでもよい）
    uploadedKeys.push(item.key);
    try {
      await storage.put(item.key, item.contentType, item.file);
    } catch (error) {
      console.error("[photos.upload] R2 put failed", error);
      await removeUnreferenced();
      return errorResponse(502, "photo storage error");
    }
  }

  // 下書きを確定してから、確定済みの投稿で同じ種類の写真がまだないときだけ行を作り、最後にこの投稿の写真をすべて読んで返す。
  // D1 の batch() は1つのトランザクションで、書き込みは1つずつ順に実行されるので、同時に送られても行は種類ごとに1つになり、
  // どのリクエストも同じ写真を返す（冪等）。Go は同時に送られると行が2組できる。
  // 保存している間に受け付けが終わったら、確定も行の作成もされず、下書きのまま残る → 409
  let photos: PhotoRow[];
  let stillDraft: boolean;
  try {
    const statements = confirmation ? [confirmation] : [];
    statements.push(...validated.map((item) =>
      db.prepare(
        `INSERT INTO photos (post_id, r2_key, photo_type)
         SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM photos WHERE post_id = ? AND photo_type = ?)
           AND EXISTS (SELECT 1 FROM posts WHERE id = ? AND is_draft = 0)`,
      ).bind(postID.value, item.key, item.photoType, postID.value, item.photoType, postID.value)
    ));
    const photosAt = statements.push(
      db.prepare("SELECT id, post_id, r2_key, photo_type FROM photos WHERE post_id = ? ORDER BY id ASC").bind(postID.value),
    ) - 1;
    const draftAt = statements.push(selectIsDraft(db, post.id)) - 1;
    const results = await db.batch(statements);
    photos = results[photosAt].results as unknown as PhotoRow[];
    stillDraft = isStillDraft(results[draftAt]);
  } catch (error) {
    // D1 は書き込めたのに応答が届かず失敗になることがある。行が指している写真は消さない
    await removeUnreferenced();
    throw error;
  }
  // 先に別のリクエストが行を作っていて拡張子が違ったときなど、どの行も指していない写真は消す
  await removeUnreferenced(photos);
  if (stillDraft) return errorResponse(409, NOT_ACCEPTING_POSTS);
  return photosResponse(storage, photos);
}

interface NotificationRow {
  sent_at: string | null;
  ended_at: string | null;
  accepting: number;
}

function selectIsDraft(db: RouteContext["db"], postID: number) {
  return db.prepare("SELECT is_draft FROM posts WHERE id = ?").bind(postID);
}

function isStillDraft(result: { results: unknown[] }): boolean {
  return (result.results[0] as { is_draft: number } | undefined)?.is_draft === 1;
}

interface FormFile {
  file: File;
  // 形式の判定に使う先頭のバイト列
  head: Uint8Array;
}

async function photosResponse(storage: PhotoStorage, photos: PhotoRow[]): Promise<Response> {
  return Response.json({ photos: await toPhotoJSONs(storage, photos) }, { status: 201 });
}

// confirmStatement は Go の confirmPhotoReport → ConfirmPhotoReport と同じく下書きを確定する（受け付けが終わっていれば確定しない）。
// 締め切りは通知から1時間後。通知が先に終わっていれば（ended_at）その時刻。
// on_time / late はリクエストを受けた時点の値で決める。保存している間に通知が終わっても on_time のまま（終わる前に送った報告なので、害はない）。
// 通知が読めなければ 500（Go は写真を保存して行を作ってから 500。外部キーがあるので起きないはず）
function confirmStatement(db: RouteContext["db"], post: PostRow, notification: NotificationRow | undefined) {
  if (!notification) throw new Error(`notification ${post.notification_id} of post ${post.id} not found`);
  const sentAt = parseGoTimestamp(notification.sent_at);
  if (!sentAt) throw new Error(`notification ${post.notification_id} has an invalid sent_at`);
  let deadline = sentAt.getTime() + CHALLENGE_WINDOW_MS;
  if (typeof notification.ended_at === "string" && notification.ended_at !== "") {
    const endedAt = parseGoTimestamp(notification.ended_at);
    if (!endedAt) throw new Error(`notification ${post.notification_id} has an invalid ended_at`);
    deadline = Math.min(deadline, endedAt.getTime());
  }
  const now = new Date();
  const status = now.getTime() > deadline ? "late" : "on_time";
  return db.prepare(
    `UPDATE posts SET is_draft = 0, status = ?, created_at = ? WHERE id = ? AND is_draft = 1 AND ${acceptsPhotoReports("?")}`,
  ).bind(status, formatRFC3339(now), post.id, post.notification_id);
}

// readMultipartForm は Go の ParseMultipartForm と同じく、multipart/form-data（boundary つき）だけを読む。
// 21MB を超える、形式が違う、読めない場合は null（Go はどれも 413 にする）。
// ボディは自分では溜めず、上限を超えたら止まるストリームのまま FormData に渡す（Workers のメモリは同じ isolate の要求で分け合うため）。
// FormData がボディを読み切ってからパートを File にするので、1件で最大ボディの2倍ほどを使いうる。
// 検証環境（R2 はスタブ）で 21MB 近いアップロードを16本同時に送り、すべて 201 になることを確かめた（2026-10-25）。
// Go と違い、壊れた Content-Type のパラメーター（"; foo" や ";" を含む引用符の値）は読めることがあり、パートの数の上限（Go は1000）もない。
// iOS は送らないので、受け入れている
async function readMultipartForm(request: Request): Promise<FormData | null> {
  const contentType = request.headers.get("Content-Type") ?? "";
  if (!isMultipartFormData(contentType)) return null;
  // Content-Length が数でなければここは通し、limitBytes で止める
  if (Number(request.headers.get("Content-Length") ?? "0") > MAX_UPLOAD_BYTES) return null;
  const body = request.body ? request.body.pipeThrough(limitBytes(MAX_UPLOAD_BYTES)) : null;
  try {
    return await new Response(body, { headers: { "Content-Type": contentType } }).formData();
  } catch {
    return null;
  }
}

// Go の ParseMultipartForm と同じく、multipart/form-data で boundary があるものだけを読む
// （同じ名前のパラメーターが2つあっても、Go は読む）
function isMultipartFormData(contentType: string): boolean {
  const [mediaType, ...parameters] = contentType.split(";");
  if (mediaType.trim().toLowerCase() !== "multipart/form-data") return false;
  return parameters.some((parameter) => {
    const [name, value = ""] = parameter.split("=", 2);
    return name.trim().toLowerCase() === "boundary" && value.trim().replace(/^"|"$/g, "") !== "";
  });
}

// readFormFile は Go の FormFile と同じく、その名前の最初のファイルを返す。
// ファイルでない値と、filename が空のパート（Go はファイルとして扱わない）は数えない
async function readFormFile(form: FormData, name: string): Promise<FormFile | null> {
  // compatibility_date が 2021-11-03 以降なので、ファイルのパートは File になる（@cloudflare/workers-types の既定の型は古い）
  const values = form.getAll(name) as unknown as (File | string)[];
  const file = values.find((value): value is File => typeof value !== "string" && value.name !== "");
  if (!file) return null;
  return { file, head: new Uint8Array(await file.slice(0, 16).arrayBuffer()) };
}

// contentTypeOf は Go の contentTypeOf と同じ。パートの Content-Type は信用せず、バイト列で判定する
// （iOS の swift-openapi-runtime は text/plain を付けるため）。heic だけは Go でも判定できないので、
// パートの Content-Type が image/* で、ftyp の brand が heic 系のときにパートの Content-Type を使う
function contentTypeOf({ file, head }: FormFile): string {
  const sniffed = sniffImage(head);
  if (sniffed) return sniffed;
  if (file.type.startsWith("image/") && looksLikeHEIC(head)) return file.type;
  return "";
}

// sniffImage は Go の http.DetectContentType が image/* を返すバイト列だけを判定する（先頭 14 バイトで足りる）
function sniffImage(data: Uint8Array): string | null {
  const startsWith = (signature: number[], offset = 0) => signature.every((byte, i) => data[offset + i] === byte);
  const ascii = (text: string) => [...text].map((ch) => ch.charCodeAt(0));
  if (startsWith([0x00, 0x00, 0x01, 0x00]) || startsWith([0x00, 0x00, 0x02, 0x00])) return "image/x-icon";
  if (startsWith(ascii("BM"))) return "image/bmp";
  if (startsWith(ascii("GIF87a")) || startsWith(ascii("GIF89a"))) return "image/gif";
  if (startsWith(ascii("RIFF")) && startsWith(ascii("WEBPVP"), 8)) return "image/webp";
  if (startsWith([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])) return "image/png";
  if (startsWith([0xff, 0xd8, 0xff])) return "image/jpeg";
  return null;
}

// looksLikeHEIC は Go と同じく、ftyp の major brand（[8..12)）が heic / heix / hevc かを見る
function looksLikeHEIC(data: Uint8Array): boolean {
  if (data.byteLength < 12) return false;
  const brand = String.fromCharCode(...data.subarray(8, 12));
  return brand === "heic" || brand === "heix" || brand === "hevc";
}
