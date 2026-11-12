import { errorResponse } from "../errors";
import { parseID } from "../params";
import { type PhotoJSON, type PhotoRow, toPhotoJSONs } from "../photos/photo";
import { photoStorage } from "../r2";
import type { RouteContext } from "../types";
import { formatRFC3339, formatTimestampNano, isGoZeroTime, parseGoTime, postColumns, type PostJSON, type PostRow, toPostJSON } from "./post";

// listPosts は GET /groups/:id/posts。
// Go の PostHandler.List → postService.ListPostsPage と同じ内容を返す。
//   下書きと missed は除く。新しい順（datetime(created_at) の降順、同じなら id の降順）
//   limit 件ずつ（既定 20、最大 50）。cursor（前のページの next_cursor）より古いものを返す
//   投稿者の login / avatar_url は、今のメンバーでなければ ""（Go の memberMap にない場合と同じ）
//   写真は id の昇順。リアクションは id の昇順で、リアクションした人の login / avatar_url つき。どちらもなければ []
//   router の認証（メンバーであること）を通った後に呼ばれる
export async function listPosts({ request, env, db, caller }: RouteContext): Promise<Response> {
  const query = new URL(request.url).searchParams;
  const limit = parseLimit(query.get("limit") ?? "");
  if (limit === null) return errorResponse(400, "invalid limit");
  const cursor = parseCursor(query.get("cursor") ?? "");
  if (cursor === null) return errorResponse(400, "invalid cursor");

  // Go の postRepository.ListPageByGroupID と同じ条件。limit+1 件読んで次のページがあるかを決める
  let page = FEED_POSTS;
  const binds: (number | string)[] = [caller.groupID];
  if (cursor !== NO_CURSOR) {
    page += " AND (datetime(p.created_at) < datetime(?) OR (datetime(p.created_at) = datetime(?) AND p.id < ?))";
    binds.push(cursor.stamp, cursor.stamp, cursor.id);
  }
  page += " ORDER BY datetime(p.created_at) DESC, p.id DESC LIMIT ?";
  const postBinds = [...binds, limit + 1];
  // 写真とリアクションは、応答する limit 件の投稿の分だけ取る（Go も limit 件に切ってから引く）。
  // 並びは id まで含めて一意なので、先頭の limit 件は投稿の側で切り詰めたものと同じになる
  const pagePostIDs = `SELECT p.id FROM posts p WHERE ${page}`;
  const relatedBinds = [...binds, limit];

  // 投稿・投稿者・写真・リアクションを1回の往復で取る。Go は写真とリアクションを IN (?, …) で引いて往復が増えるので、
  // ここではサブクエリにして同じ batch に入れる
  const [posts, photos, reactions] = await db.batch([
    db.prepare(
      `SELECT ${postColumns("p")},
              u.github_login AS login, u.avatar_url AS avatar_url
       FROM posts p
       LEFT JOIN group_members gm ON gm.group_id = p.group_id AND gm.user_id = p.user_id
       LEFT JOIN users u ON u.id = gm.user_id
       WHERE ${page}`,
    ).bind(...postBinds),
    db.prepare(
      `SELECT ph.id, ph.post_id, ph.r2_key, ph.photo_type
       FROM photos ph
       WHERE ph.post_id IN (${pagePostIDs})
       ORDER BY ph.id ASC`,
    ).bind(...relatedBinds),
    // Go の reactionRepository.ListByPostIDs と同じく、users にいない人のリアクションは出さない
    db.prepare(
      `SELECT rx.id, rx.post_id, rx.user_id, rx.reaction_type, u.github_login AS login, u.avatar_url AS avatar_url
       FROM reactions rx
       INNER JOIN users u ON rx.user_id = u.id
       WHERE rx.post_id IN (${pagePostIDs})
       ORDER BY rx.id ASC`,
    ).bind(...relatedBinds),
  ]);

  let rows = posts.results as unknown as FeedRow[];
  const hasMore = rows.length > limit;
  if (hasMore) rows = rows.slice(0, limit);

  const storage = photoStorage(env);
  const photoRows = photos.results as unknown as PhotoRow[];
  const photoJSONs = await toPhotoJSONs(storage, photoRows);
  const photosByPost = groupByPost(photoRows.map((row, i) => [row.post_id, photoJSONs[i]]));
  const reactionsByPost = groupByPost((reactions.results as unknown as ReactionRow[]).map((row) => [row.post_id, toReactionJSON(row)]));

  const result: PostFeedJSON[] = rows.map((row) => ({
    ...toPostJSON(row),
    login: typeof row.login === "string" ? row.login : "",
    avatar_url: typeof row.avatar_url === "string" ? row.avatar_url : "",
    photos: photosByPost.get(row.id) ?? [],
    reactions: reactionsByPost.get(row.id) ?? [],
  }));

  // next_cursor は次のページがあるときだけ付ける（Go の omitempty）
  const nextCursor = hasMore && rows.length > 0 ? { next_cursor: encodeCursor(rows[rows.length - 1]) } : {};
  const body: PostListResponse = { posts: result, ...nextCursor, has_more: hasMore };
  return Response.json(body);
}

// Go の postRepository.ListPageByGroupID と同じ条件（ページ送りの前の部分）
const FEED_POSTS = "p.group_id = ? AND p.is_draft = 0 AND (p.status IS NULL OR p.status != 'missed')";

const DEFAULT_LIMIT = 20;
const MAX_LIMIT = 50;

// parseLimit は Go の List と同じく、空なら 20、strconv.Atoi で読めないか 1 未満なら null、50 より大きければ 50
function parseLimit(raw: string): number | null {
  if (raw === "") return DEFAULT_LIMIT;
  const parsed = parseID(raw);
  if (!parsed.valid) return null;
  // JavaScript の数値で正確に表せない値は、int64 に収まっていれば正か負かだけ分かればよい
  const negative = raw.startsWith("-");
  if (parsed.value === null) return negative ? null : MAX_LIMIT;
  if (parsed.value < 1) return null;
  return Math.min(parsed.value, MAX_LIMIT);
}

const NO_CURSOR = Symbol("no cursor");

// Cursor は SQL に渡す値。stamp は Go と同じく、カーソルの時刻を秒までの RFC3339 にしたもの。
// id は、JavaScript の数値で正確に表せない int64 なら10進の文字列（p.id は INTEGER の列なので、SQLite が数値にして比べる）
interface Cursor {
  stamp: string;
  id: number | string;
}

const INT64_MIN = -(2n ** 63n);
const INT64_MAX = 2n ** 63n - 1n;

// parseCursor は Go の List と同じ規則でカーソルを読む。読めなければ null（400 "invalid cursor"）。
//   base64url（パディングなし）→ JSON {"created_at": RFC3339Nano, "id": 整数}
//   キーは Go の encoding/json と同じく大文字小文字を区別しない。id がなければ 0
//   id は Go の int64 と同じく、整数の表記（1.0 や 1e3 は不可）で int64 に収まるものだけ
//   空、または時刻がゼロ時刻なら、カーソルなし（最初のページ）
function parseCursor(raw: string): Cursor | typeof NO_CURSOR | null {
  if (raw === "") return NO_CURSOR;
  const decoded = decodeBase64RawURL(raw);
  if (decoded === null) return null;
  // 一番外のオブジェクトの数値の元の表記（JSON.parse の reviver は内側から順に呼ばれる）
  const numberSources = new Map<object, Map<string, string | undefined>>();
  let value: unknown;
  try {
    value = JSON.parse(decoded, function (this: object, key: string, item: unknown, context?: { source?: string }) {
      if (typeof item === "number") {
        let sources = numberSources.get(this);
        if (!sources) numberSources.set(this, (sources = new Map()));
        sources.set(key, context?.source);
      }
      return item;
    });
  } catch {
    return null;
  }
  // Go は null を読んでも構造体をゼロ値のままにする（created_at が "" なので時刻で 400 になる）
  if (value === null) return null;
  if (typeof value !== "object" || Array.isArray(value)) return null;

  let createdAt: unknown = "";
  let id: number | string = 0;
  for (const [key, field] of Object.entries(value)) {
    const name = key.toLowerCase();
    // Go は null を読むと、その項目を変えない
    if (field === null) continue;
    if (name === "created_at") {
      if (typeof field !== "string") return null;
      createdAt = field;
    } else if (name === "id") {
      const parsed = parseCursorID(field, numberSources.get(value)?.get(key));
      if (parsed === null) return null;
      id = parsed;
    }
  }

  const time = parseGoTime(createdAt, false);
  if (!time) return null;
  if (isGoZeroTime(time)) return NO_CURSOR;
  return { stamp: formatRFC3339(time.date), id };
}

// parseCursorID は、Go が int64 として読める id を、SQL に渡す値にする。読めなければ null。
// 元の表記がわからない実行環境では、JavaScript の数値で正確に表せる整数だけを受け付ける
function parseCursorID(field: unknown, source: string | undefined): number | string | null {
  if (typeof field !== "number") return null;
  if (source === undefined) return Number.isSafeInteger(field) ? field : null;
  if (!/^-?\d+$/.test(source)) return null;
  const exact = BigInt(source);
  if (exact < INT64_MIN || exact > INT64_MAX) return null;
  return Number.isSafeInteger(field) ? field : exact.toString();
}

// decodeBase64RawURL は Go の base64.RawURLEncoding.DecodeString と同じく読み、UTF-8 として文字列にする。
// = や + / は受け付けない。改行（\r \n）は読み飛ばす。読めなければ null
function decodeBase64RawURL(raw: string): string | null {
  const text = raw.replace(/[\r\n]/g, "");
  if (!/^[A-Za-z0-9_-]*$/.test(text) || text.length % 4 === 1) return null;
  const binary = atob(text.replace(/-/g, "+").replace(/_/g, "/"));
  return new TextDecoder().decode(Uint8Array.from(binary, (c) => c.charCodeAt(0)));
}

// encodeCursor は Go の List と同じく、最後の投稿の created_at（RFC3339Nano の UTC）と id から next_cursor を作る
function encodeCursor(row: PostRow): string {
  const json = JSON.stringify({ created_at: formatTimestampNano(row.created_at), id: row.id });
  const binary = String.fromCharCode(...new TextEncoder().encode(json));
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function groupByPost<T>(entries: [number, T][]): Map<number, T[]> {
  const byPost = new Map<number, T[]>();
  for (const [postID, item] of entries) {
    const list = byPost.get(postID) ?? [];
    list.push(item);
    byPost.set(postID, list);
  }
  return byPost;
}

function toReactionJSON(row: ReactionRow): ReactionJSON {
  return {
    id: row.id,
    post_id: row.post_id,
    user_id: row.user_id,
    reaction_type: row.reaction_type,
    login: typeof row.login === "string" ? row.login : "",
    avatar_url: typeof row.avatar_url === "string" ? row.avatar_url : "",
  };
}

interface FeedRow extends PostRow {
  login: string | null;
  avatar_url: string | null;
}

interface ReactionRow {
  id: number;
  post_id: number;
  user_id: number;
  reaction_type: string;
  login: string | null;
  avatar_url: string | null;
}

// Go の handler.ReactionJSON
interface ReactionJSON {
  id: number;
  post_id: number;
  user_id: number;
  reaction_type: string;
  login: string;
  avatar_url: string;
}

// Go の handler.PostFeedJSON
interface PostFeedJSON extends PostJSON {
  login: string;
  avatar_url: string;
  photos: PhotoJSON[];
  reactions: ReactionJSON[];
}

// Go の handler.PostListResponse。キーの順も Go と同じ
interface PostListResponse {
  posts: PostFeedJSON[];
  next_cursor?: string;
  has_more: boolean;
}
