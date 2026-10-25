import { type PhotoJSON, type PhotoRow, toPhotoJSONs } from "../photos/photo";
import { photoStorage } from "../r2";
import type { RouteContext } from "../types";
import { postColumns, type PostJSON, type PostRow, toPostJSON } from "./post";

// listPosts は GET /groups/:id/posts。
// Go の PostHandler.List → postService.ListPosts と同じ内容を返す。
//   下書きと missed は除く。新しい順（datetime(created_at) の降順、同じなら id の降順）
//   投稿者の login / avatar_url は、今のメンバーでなければ ""（Go の memberMap にない場合と同じ）
//   写真は id の昇順。写真がない投稿も photos は []
//   router の認証（メンバーであること）を通った後に呼ばれる
export async function listPosts({ env, db, caller }: RouteContext): Promise<Response> {
  // 投稿・投稿者・写真を1回の往復で取る。Go は写真を IN (?, …) で引くため、投稿が100件を超えると
  // D1 の上限（1つの SQL に渡せる値は100個まで）に当たって 500 になる。ここではサブクエリで引く
  const [posts, photos] = await db.batch([
    db.prepare(
      `SELECT ${postColumns("p")},
              u.github_login AS login, u.avatar_url AS avatar_url
       FROM posts p
       LEFT JOIN group_members gm ON gm.group_id = p.group_id AND gm.user_id = p.user_id
       LEFT JOIN users u ON u.id = gm.user_id
       WHERE ${FEED_POSTS}
       ORDER BY datetime(p.created_at) DESC, p.id DESC`,
    ).bind(caller.groupID),
    db.prepare(
      `SELECT ph.id, ph.post_id, ph.r2_key, ph.photo_type
       FROM photos ph
       WHERE ph.post_id IN (SELECT p.id FROM posts p WHERE ${FEED_POSTS})
       ORDER BY ph.id ASC`,
    ).bind(caller.groupID),
  ]);

  const storage = photoStorage(env);
  const photoRows = photos.results as unknown as PhotoRow[];
  const photoJSONs = await toPhotoJSONs(storage, photoRows);
  const photosByPost = new Map<number, PhotoJSON[]>();
  for (const [i, row] of photoRows.entries()) {
    const list = photosByPost.get(row.post_id) ?? [];
    list.push(photoJSONs[i]);
    photosByPost.set(row.post_id, list);
  }

  const result: PostFeedJSON[] = (posts.results as unknown as FeedRow[]).map((row) => ({
    ...toPostJSON(row),
    login: typeof row.login === "string" ? row.login : "",
    avatar_url: typeof row.avatar_url === "string" ? row.avatar_url : "",
    photos: photosByPost.get(row.id) ?? [],
  }));
  return Response.json({ posts: result });
}

// Go の postRepository.ListByGroupID と同じ条件
const FEED_POSTS = "p.group_id = ? AND p.is_draft = 0 AND (p.status IS NULL OR p.status != 'missed')";

interface FeedRow extends PostRow {
  login: string | null;
  avatar_url: string | null;
}

// Go の handler.PostFeedJSON
interface PostFeedJSON extends PostJSON {
  login: string;
  avatar_url: string;
  photos: PhotoJSON[];
}
