import { authorizeGroup, GROUP_ROLE_OWNER, GROUP_ROLE_POSTER, type GroupAccess } from "./auth";
import { MeteredDB } from "./db";
import { errorResponse } from "./errors";
import { confirmPost } from "./posts/confirm";
import { claimPhotoDraft, createPhotoDraft } from "./posts/create";
import { createReaction } from "./reactions/create";
import type { ApiEnv, RouteContext } from "./types";

interface Route {
  // WORKER_ROUTES に書く名前
  name: string;
  method: string;
  // 最初のキャプチャがグループ ID
  pattern: RegExp;
  // 認証と権限チェック。handler の前に router が行い、通った場合だけ handler を呼ぶ（書き忘れを型で防ぐため必須）
  access: GroupAccess;
  // 一部のリクエストだけを Worker で処理する API は、認証より前にここで判定する。undefined ならコンテナへ転送する。
  // 本文を読むときは request.clone() を読む（転送には元の本文を使うため）。返した値は handler の ctx.input に入る
  claim?: (request: Request) => Promise<unknown>;
  handler: (ctx: RouteContext) => Promise<Response>;
}

// Worker で処理できる API（#132）。WORKER_ROUTES に名前を挙げたものだけが有効になる。
// 1つの API は Worker か Go のどちらか一方だけが持つ。
const ROUTES: Route[] = [
  {
    name: "posts.confirm",
    method: "POST",
    pattern: /^\/groups\/([^/]+)\/posts\/([^/]+)\/confirm$/,
    access: { kind: "roles", roles: [GROUP_ROLE_OWNER, GROUP_ROLE_POSTER] },
    handler: confirmPost,
  },
  {
    // 写真の下書き（content_source = photo）だけ。GitHub / 手入力の投稿はコンテナへ転送する
    name: "posts.create",
    method: "POST",
    pattern: /^\/groups\/([^/]+)\/posts$/,
    access: { kind: "roles", roles: [GROUP_ROLE_OWNER, GROUP_ROLE_POSTER] },
    claim: claimPhotoDraft,
    handler: createPhotoDraft,
  },
  {
    name: "reactions.create",
    method: "POST",
    pattern: /^\/groups\/([^/]+)\/posts\/([^/]+)\/reactions$/,
    access: { kind: "roles", roles: [GROUP_ROLE_OWNER, GROUP_ROLE_POSTER] },
    handler: createReaction,
  },
];

// handleWorkerRoute は WORKER_ROUTES で有効にした API なら Worker で処理し、それ以外は null を返す。
// null のときは、呼び出し元がこれまでどおりコンテナへ転送する。
// 一致しない形のパス（末尾のスラッシュなど）もコンテナへ回り、Go がこれまでどおり処理する。
// パーセントエンコードを含むパスも、Go とデコードの仕方を揃えずに済むよう、すべてコンテナへ回す。
export async function handleWorkerRoute(request: Request, env: ApiEnv): Promise<Response | null> {
  const enabled = enabledRouteNames(env.WORKER_ROUTES);
  if (enabled.size === 0) return null;

  const { pathname } = new URL(request.url);
  if (pathname.includes("%")) return null;
  for (const route of ROUTES) {
    if (!enabled.has(route.name) || route.method !== request.method) continue;
    const match = route.pattern.exec(pathname);
    if (!match) continue;

    let input: unknown;
    if (route.claim) {
      try {
        input = await route.claim(request);
      } catch (error) {
        // 判定できなければ、これまでどおり Go に任せる
        console.error(`[${route.name}] claim failed`, error);
        input = undefined;
      }
      if (input === undefined) return null;
    }

    const db = new MeteredDB(env.DB);
    const params = match.slice(1);
    let response: Response;
    try {
      const caller = await authorizeGroup(request, env, db, params[0], route.access);
      response = caller instanceof Response ? caller : await route.handler({ request, env, db, params, caller, input });
    } catch (error) {
      // 予期しない失敗も、Go と同じ JSON の 500 で返す
      console.error(`[${route.name}] failed`, error);
      response = errorResponse(500, "internal server error");
    }
    return env.D1_METRICS === "true" ? db.applyMetricsHeaders(response) : response;
  }
  return null;
}

const ROUTE_NAMES = new Set(ROUTES.map((route) => route.name));
// 警告済みの WORKER_ROUTES の値（isolate ごとに1回だけ警告する）
const warnedRouteValues = new Set<string>();

export function enabledRouteNames(value: string | undefined): Set<string> {
  const names = new Set((value ?? "").split(",").map((name) => name.trim()).filter(Boolean));
  const unknown = [...names].filter((name) => !ROUTE_NAMES.has(name));
  if (unknown.length > 0 && !warnedRouteValues.has(value!)) {
    warnedRouteValues.add(value!);
    console.warn(`[router] WORKER_ROUTES has unknown names (ignored): ${unknown.join(", ")}`);
  }
  return names;
}
