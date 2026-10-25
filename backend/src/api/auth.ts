import { encryptToken } from "./crypto";
import type { MeteredDB } from "./db";
import { errorResponse } from "./errors";
import { parseID } from "./params";
import type { ApiEnv } from "./types";

export const GROUP_ROLE_OWNER = "owner";
export const GROUP_ROLE_POSTER = "poster";

export interface Caller {
  userID: number;
  groupID: number;
  // 平文のトークン（Go の ctxAccessToken。GitHub API の呼び出しに使う）
  accessToken: string;
}

// GroupAccess は Go のミドルウェアのどちらに相当するチェックをするか。
//   member : GroupMember（メンバーであればよい）
//   roles  : GroupRole（指定の role だけ。旧 role の member は poster として扱う）
export type GroupAccess = { kind: "member" } | { kind: "roles"; roles: string[] };

// authorizeGroup は Go の BearerAuth → GroupMember / GroupRole と同じ順で確かめ、同じエラーを返す。
// Go は認証とメンバー確認で2往復するが、ここでは users と group_members を1回の SQL で引く。
//   1. Authorization: Bearer <token> がない、トークンが空 → 401
//   2. トークンに一致するユーザーがいない → 401（グループ ID が不正でも、認証の失敗を先に返す）
//   3. グループ ID が整数でない → 400 "invalid group id"
//   4. メンバーでない、または role が足りない → 403
export async function authorizeGroup(
  request: Request,
  env: ApiEnv,
  db: MeteredDB,
  rawGroupID: string,
  access: GroupAccess,
): Promise<Caller | Response> {
  const header = request.headers.get("Authorization") ?? "";
  if (!header.startsWith("Bearer ")) return errorResponse(401, "unauthorized");
  const token = header.slice("Bearer ".length);
  if (token === "") return errorResponse(401, "unauthorized");

  let encrypted: string;
  try {
    encrypted = await encryptToken(env.DB_ENCRYPTION_KEY ?? "", token);
  } catch (error) {
    // 鍵の設定ミス。Go ではコンテナが起動しない状態にあたる
    console.error("[auth] DB_ENCRYPTION_KEY is invalid", error);
    return errorResponse(500, "internal server error");
  }

  const groupID = parseID(rawGroupID);
  let row: { user_id: number; role: string | null } | null;
  try {
    row = groupID.valid && groupID.value !== null
      ? await db.first(db.prepare(
        `SELECT u.id AS user_id, gm.role AS role
         FROM users u
         LEFT JOIN group_members gm ON gm.group_id = ? AND gm.user_id = u.id
         WHERE u.encrypted_access_token = ? LIMIT 1`,
      ).bind(groupID.value, encrypted))
      : await db.first(db.prepare(
        `SELECT id AS user_id, NULL AS role FROM users WHERE encrypted_access_token = ? LIMIT 1`,
      ).bind(encrypted));
  } catch (error) {
    console.error("[auth] user lookup failed", error);
    return errorResponse(500, "internal server error");
  }

  if (!row) return errorResponse(401, "unauthorized");
  if (!groupID.valid) return errorResponse(400, "invalid group id");
  // groupID.value が null（整数だが D1 に渡せない大きさ）なら、そのグループは存在しない
  if (groupID.value === null || row.role === null) return errorResponse(403, "forbidden");
  if (access.kind === "roles" && !access.roles.includes(normalizeGroupRole(row.role))) {
    return errorResponse(403, "forbidden");
  }

  return { userID: row.user_id, groupID: groupID.value, accessToken: token };
}

// normalizeGroupRole は Go の model.NormalizeGroupRole と同じく、旧 role の member を poster に読み替える。
export function normalizeGroupRole(role: string): string {
  return role === "member" ? GROUP_ROLE_POSTER : role;
}
